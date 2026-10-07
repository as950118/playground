#!/usr/bin/env python3
"""
Offline heart-rate estimate from face ROI using classical rPPG (CHROM, POS).

References:
- De Haan & Jeanne (2013), CHROM — IEEE TBME.
- Wang et al. (2016), POS — IEEE TBME; implementation structure aligned with pyVHR cpu_POS.
"""

from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

import cv2
import numpy as np
from scipy import signal


def bandpass_zero_phase(x: np.ndarray, fs: float, low_hz: float, high_hz: float, order: int = 3) -> np.ndarray:
    if fs <= 0:
        raise ValueError("fs must be positive")
    nyq = 0.5 * fs
    lo = max(low_hz / nyq, 1e-6)
    hi = min(high_hz / nyq, 0.99)
    if lo >= hi:
        raise ValueError(f"Invalid band [{low_hz}, {high_hz}] Hz for fs={fs}")
    b, a = signal.butter(order, [lo, hi], btype="band")
    return signal.filtfilt(b, a, x.astype(np.float64))


def welch_dominant_bpm(sig: np.ndarray, fs: float, bpm_min: float = 45.0, bpm_max: float = 180.0) -> tuple[float, np.ndarray, np.ndarray]:
    """Return (bpm, freqs_hz, psd) using Welch PSD peak in physiological band."""
    nperseg = min(len(sig), int(fs * 4))
    nperseg = max(nperseg, 8)
    freqs, psd = signal.welch(sig, fs=fs, nperseg=nperseg, noverlap=nperseg // 2)
    f_lo, f_hi = bpm_min / 60.0, bpm_max / 60.0
    mask = (freqs >= f_lo) & (freqs <= f_hi)
    if not np.any(mask):
        return float("nan"), freqs, psd
    idx = np.argmax(psd[mask])
    peak_f = freqs[mask][idx]
    return peak_f * 60.0, freqs, psd


def chrom_bvp(rgb: np.ndarray) -> np.ndarray:
    """
    CHROM (De Haan & Jeanne, 2013). rgb shape (N, 3) in order R, G, B float.
    Vectorization follows pyVHR cpu_CHROM for one estimator: X shape (1, N, 3).
    """
    x = rgb.astype(np.float64)[np.newaxis, :, :]
    xcomp = 3.0 * x[:, :, 0] - 2.0 * x[:, :, 1]
    ycomp = 1.5 * x[:, :, 0] + x[:, :, 1] - 1.5 * x[:, :, 2]
    sx = np.std(xcomp, axis=1, keepdims=True)
    sy = np.std(ycomp, axis=1, keepdims=True) + 1e-9
    alpha = sx / sy
    bvp = xcomp - alpha * ycomp
    return bvp[0].astype(np.float64)


def pos_bvp(rgb: np.ndarray, fs: float) -> np.ndarray:
    """
    POS (Wang et al., 2016). rgb shape (N,3) R,G,B.
    Sliding window ~1.6 s, overlap-add (pyVHR cpu_POS, e=1).
    """
    eps = 1e-9
    x = rgb.astype(np.float64).T  # (3, f)
    f = x.shape[1]
    w = max(int(1.6 * fs), 8)
    p = np.array([[0.0, 1.0, -1.0], [-2.0, 1.0, 1.0]])
    h = np.zeros(f, dtype=np.float64)
    x = x[np.newaxis, :, :]  # (1, 3, f)
    for n in range(w, f):
        m = n - w + 1
        cn = x[:, :, m : n + 1]
        mnorm = 1.0 / (np.mean(cn, axis=2) + eps)
        mnorm = np.expand_dims(mnorm, axis=2)
        cn = np.multiply(cn, mnorm)
        # P @ Cn for estimator 0: (2,3) @ (3,w) -> (2,w)
        s = np.dot(p, cn[0])
        s1 = s[0, :]
        s2 = s[1, :]
        alpha = np.std(s1) / (eps + np.std(s2))
        hn = s1 + alpha * s2
        hnm = hn - np.mean(hn)
        h[m : n + 1] += hnm
    return h


def green_bvp(rgb: np.ndarray) -> np.ndarray:
    return rgb[:, 1].astype(np.float64)


def load_face_cascade() -> cv2.CascadeClassifier:
    path = cv2.data.haarcascades + "haarcascade_frontalface_default.xml"
    cascade = cv2.CascadeClassifier(path)
    if cascade.empty():
        raise RuntimeError(f"Failed to load Haar cascade: {path}")
    return cascade


def mean_rgb_roi(frame_bgr: np.ndarray, cascade: cv2.CascadeClassifier) -> np.ndarray | None:
    gray = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2GRAY)
    faces = cascade.detectMultiScale(gray, scaleFactor=1.1, minNeighbors=5, minSize=(80, 80))
    if len(faces) == 0:
        return None
    x, y, w, h = max(faces, key=lambda r: r[2] * r[3])
    # Forehead / upper-cheek band: upper 55% of box, inset horizontally
    y1 = y + int(0.12 * h)
    y2 = y + int(0.55 * h)
    x1 = x + int(0.2 * w)
    x2 = x + int(0.8 * w)
    roi = frame_bgr[y1:y2, x1:x2]
    if roi.size == 0:
        return None
    b, g, r = cv2.split(roi)
    return np.array([np.mean(r), np.mean(g), np.mean(b)], dtype=np.float64)


def collect_rgb_stream(cap: cv2.VideoCapture, cascade: cv2.CascadeClassifier, max_frames: int | None) -> tuple[list[np.ndarray], float]:
    rgb_list: list[np.ndarray] = []
    fps = cap.get(cv2.CAP_PROP_FPS) or 30.0
    count = 0
    while True:
        ok, frame = cap.read()
        if not ok:
            break
        vec = mean_rgb_roi(frame, cascade)
        if vec is not None:
            rgb_list.append(vec)
        count += 1
        if max_frames is not None and count >= max_frames:
            break
    if len(rgb_list) < 32:
        return rgb_list, float(fps)
    # Estimate FPS from wall clock if capture doesn't report well
    reported = float(fps) if fps and fps > 1 else 30.0
    return rgb_list, reported


def run_synthetic(fs: float = 30.0, seconds: float = 20.0, bpm: float = 72.0) -> None:
    """Sanity check: synthetic sinusoidal green modulation should peak near `bpm`."""
    n = int(seconds * fs)
    t = np.arange(n) / fs
    hr_hz = bpm / 60.0
    rgb = np.zeros((n, 3))
    rgb[:, 0] = 100 + 0.5 * np.sin(2 * np.pi * hr_hz * t)
    rgb[:, 1] = 120 + 2.0 * np.sin(2 * np.pi * hr_hz * t)
    rgb[:, 2] = 90 + 0.3 * np.sin(2 * np.pi * hr_hz * t)
    for name, raw in [
        ("GREEN", green_bvp(rgb)),
        ("CHROM", chrom_bvp(rgb)),
        ("POS", pos_bvp(rgb, fs)),
    ]:
        filtered = bandpass_zero_phase(raw, fs, 0.7, 3.0)
        bpm_hat, _, _ = welch_dominant_bpm(filtered, fs)
        print(f"[synthetic] target {bpm:.1f} bpm, {name} estimate {bpm_hat:.1f} bpm")


def main() -> int:
    parser = argparse.ArgumentParser(description="rPPG offline HR (GREEN / CHROM / POS)")
    parser.add_argument("--video", type=str, default=None, help="Path to video file")
    parser.add_argument("--camera", type=int, default=None, help="Camera index (e.g. 0)")
    parser.add_argument("--seconds", type=float, default=20.0, help="Max seconds when using camera")
    parser.add_argument("--synthetic", action="store_true", help="Run built-in synthetic signal test and exit")
    args = parser.parse_args()

    if args.synthetic:
        run_synthetic()
        return 0

    if (args.video is None) == (args.camera is None):
        print("Specify exactly one of --video PATH or --camera N", file=sys.stderr)
        return 2

    cascade = load_face_cascade()
    if args.camera is not None:
        cap = cv2.VideoCapture(args.camera)
        max_frames = int(args.seconds * 30)
    else:
        path = Path(args.video)
        if not path.is_file():
            print(f"Video not found: {path}", file=sys.stderr)
            return 1
        cap = cv2.VideoCapture(str(path))
        max_frames = None

    if not cap.isOpened():
        print("Could not open video source", file=sys.stderr)
        return 1

    t0 = time.perf_counter()
    rgb_list, fs = collect_rgb_stream(cap, cascade, max_frames)
    cap.release()
    elapsed = time.perf_counter() - t0

    if len(rgb_list) < int(2 * fs):
        print(f"Too few valid face frames ({len(rgb_list)}). Try longer clip or better lighting.", file=sys.stderr)
        return 1

    rgb = np.stack(rgb_list, axis=0)
    print(f"Frames with face: {len(rgb)} (~{len(rgb)/max(fs,1e-6):.1f}s at fs≈{fs:.1f} Hz), read time {elapsed:.2f}s")

    results: dict[str, float] = {}
    for name, raw in [("GREEN", green_bvp(rgb)), ("CHROM", chrom_bvp(rgb)), ("POS", pos_bvp(rgb, fs))]:
        filtered = bandpass_zero_phase(raw, fs, 0.7, 3.5)
        bpm_hat, _, _ = welch_dominant_bpm(filtered, fs)
        results[name] = bpm_hat
        print(f"  {name}: {bpm_hat:.1f} bpm")

    spread = max(results.values()) - min(results.values())
    if not np.isnan(spread):
        print(f"  spread(max-min): {spread:.1f} bpm (large spread → lighting/motion/FPS issues likely)")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
