"""
Trained anti-spoofing check (MiniFASNet), as an additional signal alongside
liveness.py's classical heuristic — not a replacement for it.

Where this came from and why it's trustworthy: these are the original
weights from minivision-ai/Silent-Face-Anti-Spoofing on GitHub (Apache-2.0,
1800+ stars, actively maintained company, not an anonymous upload) —
downloaded directly from that repo, sha256-recorded at the time, and
converted to ONNX here by hand rather than trusting a third party's
pre-converted file, specifically so every step of how these weights became
this ONNX file is something that was actually run and checked, not taken on
faith. The conversion was verified numerically against the original PyTorch
model (max output difference ~1e-6 — floating-point noise, not a real
divergence), and the full pipeline below (crop, preprocess, ensemble,
decision) was verified to reproduce the original unmodified code's output
exactly on a real test photo.

One real gotcha worth recording: the original repo's own ToTensor docstring
claims pixel values get scaled to [0,1], but the actual code has that
division commented out — the model was trained on raw [0,255] values. Using
the documented [0,1] scaling (an easy, reasonable-looking assumption)
silently produced confidently wrong results on every real photo tested.
Caught only by comparing against the original code's own output, not by
reading the docs — see git history for this file if that mismatch needs
re-confirming later.

Unlike liveness.py, this needs no external detector of its own — it reuses
the bbox the embedding pipeline already computed from InsightFace's
detector; bbox format ([x, y, w, h] vs [x1,y1,x2,y2]) is the one thing this
module's own caller must get right.

Known limitation carried over from the classical heuristic, now mostly
closed: this model is trained specifically on print and screen-replay
attacks (the two classes besides "real" are exactly those), the same two
attacks liveness.py targets — but liveness.py's screen-detection is largely
blind to a plain printed photo, while this model is trained on both. This
is why the two are combined (both must agree it's live) rather than one
replacing the other.
"""

from __future__ import annotations

import os

import cv2
import numpy as np
import onnxruntime as ort

_MODELS_DIR = os.path.join(os.path.dirname(__file__), "..", "models")

# (onnx filename, crop scale) — the scale is baked into each model's own
# training data (how much context around the face bbox it expects to see),
# not a tunable parameter; mirrors the original repo's own file-naming
# convention (e.g. "2.7_80x80_...") exactly.
_MODEL_SPECS = [
    ("minifasnet_v2_2.7_80x80.onnx", 2.7),
    ("minifasnet_v1se_4.0_80x80.onnx", 4.0),
]

_OUT_SIZE = 80

# NOTE: this is NOT "accept if real_prob >= 0.5". The original repo's own
# decision rule is argmax over the 3 classes (real=1 wins whenever it's the
# plurality, even at e.g. 0.45 vs 0.30/0.25) — a flat 0.5 cutoff on
# real_prob is stricter than that and was caught, via real-photo testing,
# producing a false rejection on a real employee under flat/overexposed
# lighting (real_prob 0.449, still the clear plurality over both fake
# classes). compute_minifas_score's real_prob already reflects this: it's
# compared against the OTHER two classes below, not against a flat number.

_sessions: list[tuple[ort.InferenceSession, float]] | None = None


def _get_sessions() -> list[tuple[ort.InferenceSession, float]]:
    global _sessions
    if _sessions is None:
        _sessions = [
            (ort.InferenceSession(os.path.join(_MODELS_DIR, name), providers=["CPUExecutionProvider"]), scale)
            for name, scale in _MODEL_SPECS
        ]
    return _sessions


def warm_up():
    """Loads both ONNX sessions once at startup rather than on the first
    request — same reasoning as the main embedding model's own warm-up."""
    _get_sessions()


def _crop_scaled(img: np.ndarray, bbox_xywh: list[int], scale: float, out_size: int) -> np.ndarray:
    """Crops a `scale`x-enlarged, centered square around bbox_xywh and
    resizes to out_size x out_size — a direct port of the original repo's
    own CropImage.crop/_get_new_box, since the model's own crop-scale
    calibration depends on matching this exactly, not an approximation of
    it."""
    src_h, src_w = img.shape[:2]
    x, y, box_w, box_h = bbox_xywh

    scale = min((src_h - 1) / box_h, min((src_w - 1) / box_w, scale))
    new_w, new_h = box_w * scale, box_h * scale
    cx, cy = box_w / 2 + x, box_h / 2 + y

    left, top = cx - new_w / 2, cy - new_h / 2
    right, bottom = cx + new_w / 2, cy + new_h / 2

    if left < 0:
        right -= left
        left = 0
    if top < 0:
        bottom -= top
        top = 0
    if right > src_w - 1:
        left -= right - src_w + 1
        right = src_w - 1
    if bottom > src_h - 1:
        top -= bottom - src_h + 1
        bottom = src_h - 1

    left, top, right, bottom = int(left), int(top), int(right), int(bottom)
    crop = img[top : bottom + 1, left : right + 1]
    return cv2.resize(crop, (out_size, out_size))


def compute_minifas_score(img_bgr: np.ndarray, bbox_xyxy) -> tuple[bool, dict]:
    """bbox_xyxy: (x1, y1, x2, y2), the same format InsightFace's face.bbox
    already provides — converted to [x, y, w, h] internally to match what
    the original crop logic expects.
    Returns (is_real, diagnostics) — is_real follows the original repo's own
    argmax decision rule (real wins if it's the plurality of the 3 classes,
    not necessarily >50% of the total probability mass; see MINIFAS_THRESHOLD
    comment above for why a flat 0.5 cutoff is wrong here)."""
    x1, y1, x2, y2 = bbox_xyxy
    bbox_xywh = [int(x1), int(y1), int(x2 - x1), int(y2 - y1)]

    prediction = np.zeros(3, dtype=np.float32)
    for sess, scale in _get_sessions():
        patch = _crop_scaled(img_bgr, bbox_xywh, scale, _OUT_SIZE)
        # Deliberately NOT /255 — see module docstring. BGR kept as-is
        # (OpenCV's native order), matching what these weights were trained on.
        inp = patch.astype(np.float32).transpose(2, 0, 1)[None, ...]
        out = sess.run(None, {"input": inp})[0][0]
        exp = np.exp(out - out.max())
        prediction += exp / exp.sum()

    prediction /= len(_MODEL_SPECS)
    is_real = bool(int(np.argmax(prediction)) == 1)
    return is_real, {
        "fake_print_prob": float(prediction[0]),
        "real_prob": float(prediction[1]),
        "fake_replay_prob": float(prediction[2]),
    }
