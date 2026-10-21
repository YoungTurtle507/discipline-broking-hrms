"""Passive, single-frame liveness heuristic.

This is NOT a trained anti-spoofing model — it's classical computer-vision
signal analysis, chosen deliberately over pulling in an unverified pretrained
model file from the internet (no way to check its provenance or validate its
accuracy in this environment). It targets the exact attack it was written to
catch: a printed photo or a phone/tablet screen held up to the camera.

Two independent signals, combined into one score:

1. Frequency-domain periodicity ("moire" score). A photograph OF A SCREEN
   picks up that screen's pixel grid and any moire interference between the
   two grids (the display's and the capturing camera's sensor). That shows up
   as sharp, concentrated peaks in the image's frequency spectrum. Real skin
   texture has no such regular structure, so its spectral energy is spread
   out much more evenly. This signal is largely blind to a PRINTED photo
   (paper has no pixel grid to alias against), which is a known limitation —
   see the module-level docstring in main.py.

2. Specular highlight ("glare") ratio. A screen reflects or emits light very
   differently from skin — under almost any ambient light, a phone/tablet
   showing a photo tends to blow out a noticeably larger patch of
   near-white pixels than a real face does. A little glare is normal
   (glasses, oily skin, a bright window), so this alone is deliberately a
   soft, secondary signal, not a hard cutoff.

Thresholds below are reasoned starting points, not empirically calibrated —
there's no labeled real-vs-spoof dataset available to tune against here.
Expect to adjust LIVENESS_THRESHOLD (and the two normalization ranges) after
real testing: hold up the exact phone-photo spoof that prompted this, and
separately confirm real employees (including ones who wear glasses, or are
scanning near a bright window) aren't getting wrongly rejected.
"""

import cv2
import numpy as np

# Below this, /embed rejects the image outright as a likely photo/screen
# replay rather than proceeding to face matching at all.
LIVENESS_THRESHOLD = 0.40

# How much weight each signal gets in the combined score. Periodicity is
# weighted higher because it's the more specific signal for "this is a photo
# of a screen" — glare is easier to trigger by accident (glasses, lighting).
_FREQ_WEIGHT = 0.6
_GLARE_WEIGHT = 0.4

_ANALYSIS_SIZE = 128
_DC_MASK_RADIUS = 12
_MIN_ENERGY_FOR_PERIODICITY = 5.0


def compute_liveness_score(img: np.ndarray, bbox) -> tuple[float, dict]:
    """Returns (score in [0,1], diagnostics) for the face region in bbox.
    Higher = more likely a live face; lower = more likely a photo/screen
    replay. diagnostics is returned so a rejection can be logged with enough
    detail to help re-tune the thresholds later without needing to reproduce
    the exact scan.
    """
    h, w = img.shape[:2]
    x1, y1, x2, y2 = bbox
    x1, y1 = max(0, int(x1)), max(0, int(y1))
    x2, y2 = min(w, int(x2)), min(h, int(y2))
    face = img[y1:y2, x1:x2]
    if face.size == 0:
        return 0.0, {"error": "empty face crop"}

    gray = cv2.cvtColor(face, cv2.COLOR_BGR2GRAY)
    gray = cv2.resize(gray, (_ANALYSIS_SIZE, _ANALYSIS_SIZE)).astype(np.float32)

    freq_score, periodicity_ratio = _periodicity_score(gray)
    glare_score, bright_ratio = _glare_score(face)

    score = _FREQ_WEIGHT * freq_score + _GLARE_WEIGHT * glare_score
    return float(np.clip(score, 0.0, 1.0)), {
        "periodicity_ratio": periodicity_ratio,
        "bright_ratio": bright_ratio,
        "freq_score": freq_score,
        "glare_score": glare_score,
    }


def _periodicity_score(gray: np.ndarray) -> tuple[float, float]:
    spectrum = np.fft.fftshift(np.fft.fft2(gray))
    magnitude = np.abs(spectrum)

    center = _ANALYSIS_SIZE // 2
    yy, xx = np.ogrid[:_ANALYSIS_SIZE, :_ANALYSIS_SIZE]
    dc_mask = (yy - center) ** 2 + (xx - center) ** 2 <= _DC_MASK_RADIUS**2
    high_freq = magnitude.copy()
    high_freq[dc_mask] = 0

    mean_energy = float(high_freq.mean())

    # A near-flat/very-smooth crop (little high-frequency content at all —
    # possible on a soft-focus or low-detail capture) has almost nothing to
    # divide by, which would otherwise make the ratio below swing wildly on
    # noise alone. With nothing to measure periodicity against, there's
    # nothing to penalize either — treat it as neutral.
    if mean_energy < _MIN_ENERGY_FOR_PERIODICITY:
        return 1.0, 0.0

    # A screen's pixel grid/moire concentrates almost all its energy into a
    # handful of sharp, isolated spectral points (often just a few pixels out
    # of 16384) — a percentile misses that entirely, since 99.5% still lands
    # in the near-zero background around such a small spike. max() is the
    # right tool for "is there an isolated spike at all."
    peak_energy = float(high_freq.max())
    periodicity_ratio = peak_energy / mean_energy

    # Reasoned starting range from synthetic-only testing (no labeled
    # real-face/spoof dataset was available — see module docstring): random
    # natural-looking texture measured well under 50; a screen's pixel grid
    # or moire measured in the thousands. The gap between those two is wide
    # enough that this range is deliberately conservative rather than tight.
    freq_score = float(np.clip(1.0 - (periodicity_ratio - 50.0) / 450.0, 0.0, 1.0))
    return freq_score, periodicity_ratio


def _glare_score(face_bgr: np.ndarray) -> tuple[float, float]:
    hsv = cv2.cvtColor(face_bgr, cv2.COLOR_BGR2HSV)
    brightness = hsv[:, :, 2]
    bright_ratio = float(np.mean(brightness > 245))

    # A couple of percent near-white is unremarkable (glasses reflection, a
    # bright highlight on the forehead/nose); a meaningfully larger patch is
    # the deliberately soft cue here.
    glare_score = float(np.clip(1.0 - (bright_ratio - 0.02) / 0.08, 0.0, 1.0))
    return glare_score, bright_ratio
