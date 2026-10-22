import logging
import os

import cv2
import numpy as np
import onnxruntime as ort
from fastapi import FastAPI, File, HTTPException

# onnxruntime's default session settings favor speed over memory: the CPU
# memory arena pre-grows and holds onto blocks rather than freeing them, and
# the memory-pattern planner caches a reuse plan per input shape — wasted
# here since our inputs (face crops) vary in size every request. Patching
# this before minifas.py ever constructs a session measurably reduces peak
# RSS with no change to outputs (verified: this only changes allocation
# strategy, not computation).
_original_ort_init = ort.InferenceSession.__init__


def _low_memory_ort_init(self, path_or_bytes, sess_options=None, **kwargs):
    if sess_options is None:
        sess_options = ort.SessionOptions()
        sess_options.enable_cpu_mem_arena = False
        sess_options.enable_mem_pattern = False
        sess_options.intra_op_num_threads = 1
        sess_options.inter_op_num_threads = 1
    _original_ort_init(self, path_or_bytes, sess_options=sess_options, **kwargs)


ort.InferenceSession.__init__ = _low_memory_ort_init

from .liveness import LIVENESS_THRESHOLD, compute_liveness_score  # noqa: E402
from .minifas import compute_minifas_score, warm_up as warm_up_minifas  # noqa: E402

# Without this, a plain getLogger(...).warning(...) call has nothing attached
# to actually write it anywhere — it silently goes nowhere rather than
# reaching logs/face.log, which is exactly what happened to every rejection
# logged before this line existed (verified: none of them showed up).
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("face_service")

app = FastAPI(title="HR System Face Service")

MODEL_NAME = "opencv-yunet-detection-only"
MODEL_VERSION = "2023mar"
MIN_DETECTION_SCORE = 0.70  # YuNet's own confidence score, not comparable to InsightFace's det_score scale

_YUNET_MODEL_PATH = os.path.join(os.path.dirname(__file__), "..", "models", "face_detection_yunet.onnx")

_detector: cv2.FaceDetectorYN | None = None


def get_detector() -> cv2.FaceDetectorYN:
    global _detector
    if _detector is None:
        # Attendance marking no longer identifies who's in frame by face
        # match (the employee is picked explicitly before the camera opens —
        # see the Go backend's Mark handler) — only "is there one real, live
        # face here" matters now. That dropped InsightFace's whole
        # FaceAnalysis pipeline entirely, not just its recognition model:
        # measurement showed InsightFace's own detector (SCRFD-10GF,
        # det_10g.onnx) was responsible for nearly all of this process's
        # ~700MB peak memory on its own — recognition barely added anything
        # on top of it. The weight file (17MB) looked small, but SCRFD-10GF
        # is a computationally heavy architecture (10 GFLOPs at 640x640),
        # and that's what drove onnxruntime's memory, not the file size.
        # YuNet is a detector built for exactly this constraint (it's what
        # OpenCV ships for lightweight face detection) — 227KB on disk, runs
        # through OpenCV's own DNN engine (not onnxruntime), and measured at
        # well under 250MB total for this whole service including the two
        # anti-spoofing models below.
        _detector = cv2.FaceDetectorYN.create(_YUNET_MODEL_PATH, "", (320, 320))
    return _detector


@app.on_event("startup")
def warm_up_model():
    # Loading the model pack takes a moment; do it once at startup rather
    # than on the first request.
    get_detector()
    warm_up_minifas()


@app.get("/health")
def health():
    return {"status": "ok"}


def decode_image(raw: bytes) -> np.ndarray:
    buf = np.frombuffer(raw, dtype=np.uint8)
    img = cv2.imdecode(buf, cv2.IMREAD_COLOR)
    if img is None:
        raise HTTPException(status_code=400, detail="could not decode image")
    return img


@app.post("/verify-capture")
# A plain `def` (not `async def`) — FastAPI runs a sync path function in
# Starlette's worker threadpool instead of directly on the event loop. The
# body below is pure CPU-bound work (onnxruntime inference, OpenCV, numpy),
# which used to run inline on the single event-loop thread and block every
# other in-flight request (including /health) until it finished; this way
# concurrent scans (e.g. two kiosks) actually overlap instead of serializing.
def verify_capture(file: bytes = File(...)):
    img = decode_image(file)

    detector = get_detector()
    h, w = img.shape[:2]
    detector.setInputSize((w, h))
    _, detections = detector.detect(img)

    if detections is None or len(detections) == 0:
        logger.warning("rejected: no face detected")
        raise HTTPException(status_code=422, detail="no face detected in image")
    if len(detections) > 1:
        logger.warning("rejected: %d faces detected", len(detections))
        raise HTTPException(status_code=422, detail="multiple faces detected; only one person should be in frame")

    det = detections[0]
    det_score = float(det[14])
    if det_score < MIN_DETECTION_SCORE:
        logger.warning("rejected: detection confidence %.3f below %.3f", det_score, MIN_DETECTION_SCORE)
        raise HTTPException(status_code=422, detail="face detection confidence too low; retake in better lighting")

    x, y, box_w, box_h = det[0:4]
    bbox = (float(x), float(y), float(x + box_w), float(y + box_h))

    # Passive liveness check — rejects a printed photo or a phone/tablet
    # screen held up to the camera. See liveness.py for exactly what this
    # does and does not catch, and why the thresholds there are a
    # starting point rather than a calibrated cutoff.
    liveness_score, liveness_debug = compute_liveness_score(img, bbox)
    if liveness_score < LIVENESS_THRESHOLD:
        # Logged with the full diagnostic breakdown specifically so a false
        # rejection (a real employee) can be told apart from a real catch (an
        # actual spoof) after the fact, without needing to reproduce it —
        # that's what re-tuning LIVENESS_THRESHOLD and the two signal ranges
        # in liveness.py will need.
        logger.warning("liveness rejection: score=%.3f %s", liveness_score, liveness_debug)
        raise HTTPException(
            status_code=422,
            detail="liveness check failed — this looks like a photo or screen rather than a live face. "
            "Please scan directly with your face, not a photo of it.",
        )

    # Trained anti-spoofing model, checked independently of the classical
    # heuristic above — both must agree the face is live. The classical
    # check above is largely blind to a plain printed photo (no screen grid
    # to alias against); this model is trained on exactly that attack as
    # well as screen replay, so it closes that gap rather than duplicating
    # the same check. See minifas.py for where this model came from and how
    # it was verified.
    minifas_real, minifas_debug = compute_minifas_score(img, bbox)
    if not minifas_real:
        logger.warning("anti-spoofing rejection: %s", minifas_debug)
        raise HTTPException(
            status_code=422,
            detail="anti-spoofing check failed — this looks like a photo or screen rather than a live face. "
            "Please scan directly with your face, not a photo of it.",
        )

    return {
        "quality_score": det_score,
        "liveness_score": liveness_score,
        "minifas_real_prob": minifas_debug["real_prob"],
        "model_name": MODEL_NAME,
        "model_version": MODEL_VERSION,
    }
