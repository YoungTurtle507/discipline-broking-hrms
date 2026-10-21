"use client";

import { useEffect, useRef, useState, type RefObject } from "react";

export function captureFrameBlob(video: HTMLVideoElement | null, canvas: HTMLCanvasElement | null): Promise<Blob | null> {
  return new Promise((resolve) => {
    if (!video || !canvas || video.videoWidth === 0) {
      resolve(null);
      return;
    }
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      resolve(null);
      return;
    }
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
    canvas.toBlob((blob) => resolve(blob), "image/jpeg", 0.92);
  });
}

// Requests the front-facing camera and attaches the stream to videoRef.
// Callers should keep the <video> element mounted for the component's whole
// lifetime — swapping it out (e.g. for a captured-photo preview) loses the
// stream binding on remount since this effect only runs once.
export function useCameraStream(videoRef: RefObject<HTMLVideoElement | null>) {
  const streamRef = useRef<MediaStream | null>(null);
  const [cameraError, setCameraError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    if (!navigator.mediaDevices?.getUserMedia) {
      setCameraError(
        "Camera access isn't available on this connection. Browsers only allow it over HTTPS or from localhost — open this app via http://localhost:3000 (not a LAN IP) to use the camera."
      );
      return;
    }

    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: "user", width: 640, height: 640 } })
      .then((stream) => {
        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) videoRef.current.srcObject = stream;
      })
      .catch(() => setCameraError("Could not access the camera. Check browser permissions."));

    return () => {
      cancelled = true;
      streamRef.current?.getTracks().forEach((t) => t.stop());
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return cameraError;
}
