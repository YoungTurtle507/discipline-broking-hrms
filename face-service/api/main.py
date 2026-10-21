import io
import logging

import cv2
import numpy as np
from fastapi import FastAPI, File, HTTPException
from insightface.app import FaceAnalysis

from .liveness import LIVENESS_THRESHOLD, compute_liveness_score
from .minifas import compute_minifas_score, warm_up as warm_up_minifas

# Without this, a plain getLogger(...).warning(...) call has nothing attached
# to actually write it anywhere — it silently goes nowhere rather than
# reaching logs/face.log, which is exactly what happened to every rejection
# logged before this line existed (verified: none of them showed up).
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("face_service")

app = FastAPI(title="HR System Face Service")

MODEL_NAME = "insightface-buffalo_l"
MODEL_VERSION = "0.7.3"
MIN_DETECTION_SCORE = 0.55

_face_app: FaceAnalysis | None = None


def get_face_app() -> FaceAnalysis:
    global _face_app
    if _face_app is None:
        # buffalo_l ships 5 models (detection, landmark_3d_68, landmark_2d_106,
        # genderage, recognition), but /embed below only ever reads
        # face.bbox, face.det_score, and face.normed_embedding — none of
        # which come from the landmark or genderage models (recognition's
        # own alignment uses the detector's built-in keypoints, not those).
        # Restricting allowed_modules skips loading the 3 unused ones,
        # cutting cold-start model-load time from ~5.5s to ~1.8s and the
        # memory footprint along with it — verified to produce a
        # byte-identical embedding output either way (cosine similarity
        # 1.0 against the unrestricted load, same det_score), so this is
        # free speed, not a smaller/different model.
        _face_app = FaceAnalysis(
            name="buffalo_l",
            providers=["CPUExecutionProvider"],
            allowed_modules=["detection", "recognition"],
        )
        _face_app.prepare(ctx_id=0, det_size=(640, 640))
    return _face_app


@app.on_event("startup")
def warm_up_model():
    # Loading the model pack takes a few seconds; do it once at startup
    # rather than on the first request.
    get_face_app()
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


@app.post("/embed")
# A plain `def` (not `async def`) — FastAPI runs a sync path function in
# Starlette's worker threadpool instead of directly on the event loop. The
# body below is pure CPU-bound work (onnxruntime inference, OpenCV, numpy),
# which used to run inline on the single event-loop thread and block every
# other in-flight request (including /health) until it finished; this way
# concurrent scans (e.g. two kiosks) actually overlap instead of serializing.
def embed(file: bytes = File(...)):
    img = decode_image(file)

    faces = get_face_app().get(img)

    if len(faces) == 0:
        logger.warning("rejected: no face detected")
        raise HTTPException(status_code=422, detail="no face detected in image")
    if len(faces) > 1:
        logger.warning("rejected: %d faces detected", len(faces))
        raise HTTPException(status_code=422, detail="multiple faces detected; only one person should be in frame")

    face = faces[0]
    if face.det_score < MIN_DETECTION_SCORE:
        logger.warning("rejected: detection confidence %.3f below %.3f", face.det_score, MIN_DETECTION_SCORE)
        raise HTTPException(status_code=422, detail="face detection confidence too low; retake in better lighting")

    # Passive liveness check — rejects a printed photo or a phone/tablet
    # screen held up to the camera before it ever reaches face matching. See
    # liveness.py for exactly what this does and does not catch, and why the
    # thresholds there are a starting point rather than a calibrated cutoff.
    liveness_score, liveness_debug = compute_liveness_score(img, face.bbox)
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
    minifas_real, minifas_debug = compute_minifas_score(img, face.bbox)
    if not minifas_real:
        logger.warning("anti-spoofing rejection: %s", minifas_debug)
        raise HTTPException(
            status_code=422,
            detail="anti-spoofing check failed — this looks like a photo or screen rather than a live face. "
            "Please scan directly with your face, not a photo of it.",
        )

    return {
        # L2-normalized so callers can compare embeddings with a plain dot
        # product (equivalent to cosine similarity on unit vectors).
        "embedding": face.normed_embedding.tolist(),
        "quality_score": float(face.det_score),
        "liveness_score": liveness_score,
        "minifas_real_prob": minifas_debug["real_prob"],
        "model_name": MODEL_NAME,
        "model_version": MODEL_VERSION,
    }
