"use client";

import { use, useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Camera,
  CheckCircle,
  ArrowsClockwise,
  X,
  WarningCircle,
  ScanSmiley,
} from "@phosphor-icons/react";
import { apiFetch, apiUpload, ApiError } from "@/lib/api";
import { captureFrameBlob, useCameraStream } from "@/lib/camera";
import {
  FACE_SAMPLES,
  type Employee,
  type FaceSample,
  type FaceSampleType,
  type FaceVerifyResult,
} from "../../_components/types";
import { formatDate } from "../../../_components/my/types";

type CaptureAction =
  | { kind: "register"; sampleType: FaceSampleType; label: string }
  | { kind: "verify" };

export default function FaceRegistrationPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [samples, setSamples] = useState<FaceSample[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [action, setAction] = useState<CaptureAction | null>(null);

  async function load() {
    try {
      const [emp, sampleData] = await Promise.all([
        apiFetch<Employee>(`/api/v1/employees/${id}`),
        apiFetch<{ samples: FaceSample[] | null }>(`/api/v1/employees/${id}/face-samples`),
      ]);
      setEmployee(emp);
      setSamples(sampleData.samples ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load employee");
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  function sampleFor(type: FaceSampleType): FaceSample | undefined {
    return samples.find((s) => s.sample_type === type);
  }

  if (error && !employee) return <p className="text-sm text-error">{error}</p>;
  if (!employee) return <p className="text-sm text-slate-400">Loading&hellip;</p>;

  const hasAnyRegistration = samples.length > 0;

  return (
    <div>
      <Link
        href={`/dashboard/employees/${id}`}
        className="mb-4 inline-flex items-center gap-2 text-sm font-medium text-slate-500 hover:text-slate-700"
      >
        <ArrowLeft size={16} />
        Back to Profile
      </Link>

      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Face Registration</h1>
          <p className="mt-1 text-sm text-slate-500">
            Register {employee.full_name}&apos;s face samples for attendance verification.
          </p>
        </div>
        <button
          type="button"
          disabled={!hasAnyRegistration}
          onClick={() => setAction({ kind: "verify" })}
          className="flex items-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
          title={hasAnyRegistration ? "" : "Register at least the front-facing sample first"}
        >
          <ScanSmiley size={16} />
          Test Recognition
        </button>
      </div>

      {error && <p className="mb-4 text-sm text-error">{error}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {FACE_SAMPLES.map((req) => {
          const sample = sampleFor(req.type);
          return (
            <div key={req.type} className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
              <div className="mb-2 flex items-center justify-between">
                <h2 className="text-base font-semibold text-slate-900">
                  {req.label}
                  {!req.required && <span className="ml-1.5 text-xs font-normal text-slate-400">(Optional)</span>}
                </h2>
                {sample ? (
                  <span className="flex items-center gap-1 text-xs font-medium text-success">
                    <CheckCircle size={16} weight="fill" />
                    Registered
                  </span>
                ) : (
                  <span className="text-xs font-medium text-slate-400">Not registered</span>
                )}
              </div>
              <p className="mb-4 text-sm text-slate-500">{req.instructions}</p>
              {sample && (
                <p className="mb-3 text-xs text-slate-400">
                  Quality {(sample.quality_score * 100).toFixed(0)}% &middot; captured{" "}
                  {formatDate(sample.created_at)}
                </p>
              )}
              <button
                type="button"
                onClick={() => setAction({ kind: "register", sampleType: req.type, label: req.label })}
                className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                {sample ? <ArrowsClockwise size={16} /> : <Camera size={16} />}
                {sample ? "Re-register" : "Capture"}
              </button>
            </div>
          );
        })}
      </div>

      {action?.kind === "register" && (
        <RegisterModal
          employeeId={id}
          sampleType={action.sampleType}
          label={action.label}
          onClose={() => setAction(null)}
          onRegistered={() => {
            setAction(null);
            load();
          }}
        />
      )}

      {action?.kind === "verify" && (
        <VerifyModal employeeId={id} employeeName={employee.full_name} onClose={() => setAction(null)} />
      )}
    </div>
  );
}

function RegisterModal({
  employeeId,
  sampleType,
  label,
  onClose,
  onRegistered,
}: {
  employeeId: string;
  sampleType: FaceSampleType;
  label: string;
  onClose: () => void;
  onRegistered: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const cameraError = useCameraStream(videoRef);
  const [snapshotUrl, setSnapshotUrl] = useState<string | null>(null);
  const [snapshot, setSnapshot] = useState<Blob | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);

  async function capture() {
    const blob = await captureFrameBlob(videoRef.current, canvasRef.current);
    if (!blob) return;
    setSnapshot(blob);
    setSnapshotUrl(URL.createObjectURL(blob));
  }

  function retake() {
    setSnapshot(null);
    if (snapshotUrl) URL.revokeObjectURL(snapshotUrl);
    setSnapshotUrl(null);
    setUploadError(null);
  }

  async function confirm() {
    if (!snapshot) return;
    setUploading(true);
    setUploadError(null);
    try {
      const formData = new FormData();
      formData.append("sample_type", sampleType);
      formData.append("file", snapshot, "capture.jpg");
      await apiUpload(`/api/v1/employees/${employeeId}/face-samples`, formData);
      onRegistered();
    } catch (err) {
      setUploadError(err instanceof ApiError ? err.message : "Failed to register face sample");
      setUploading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">{label}</h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        <div className="relative aspect-square w-full overflow-hidden rounded-lg bg-slate-900">
          {cameraError && (
            <div className="absolute inset-0 z-10 flex items-center justify-center bg-slate-900 p-4 text-center text-sm text-slate-300">
              <WarningCircle size={18} className="mr-2 shrink-0" />
              {cameraError}
            </div>
          )}
          {/* The video element stays mounted for the modal's whole lifetime —
              swapping it out for an <img> on capture (and back on retake)
              would lose the stream's srcObject binding on remount. */}
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            className={`h-full w-full object-cover ${snapshotUrl ? "hidden" : ""}`}
          />
          {snapshotUrl && (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={snapshotUrl} alt="Captured sample" className="absolute inset-0 h-full w-full object-cover" />
          )}
        </div>
        <canvas ref={canvasRef} className="hidden" />

        {uploadError && <p className="mt-3 text-sm text-error">{uploadError}</p>}

        <div className="mt-4 flex gap-2">
          {!snapshot ? (
            <button
              type="button"
              disabled={!!cameraError}
              onClick={capture}
              className="flex-1 flex items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:opacity-50"
            >
              <Camera size={16} />
              Capture
            </button>
          ) : (
            <>
              <button
                type="button"
                disabled={uploading}
                onClick={confirm}
                className="flex-1 flex items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:opacity-70"
              >
                {uploading ? "Uploading…" : "Confirm & Upload"}
              </button>
              <button
                type="button"
                disabled={uploading}
                onClick={retake}
                className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                Retake
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

const SCAN_INTERVAL_MS = 1500;

function VerifyModal({
  employeeId,
  employeeName,
  onClose,
}: {
  employeeId: string;
  employeeName: string;
  onClose: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const cameraError = useCameraStream(videoRef);
  const checkingRef = useRef(false);
  const [result, setResult] = useState<FaceVerifyResult | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);

  useEffect(() => {
    if (cameraError || result) return;

    const interval = setInterval(async () => {
      if (checkingRef.current) return;
      checkingRef.current = true;
      try {
        const blob = await captureFrameBlob(videoRef.current, canvasRef.current);
        if (!blob) return;
        const formData = new FormData();
        formData.append("file", blob, "scan.jpg");
        const res = await apiUpload<FaceVerifyResult>(`/api/v1/employees/${employeeId}/face-verify`, formData);
        setScanError(null);
        if (res.matched) setResult(res);
      } catch (err) {
        // A 422 just means "no clear face in frame right now" — expected
        // while the person is getting positioned, so stay quiet and keep
        // trying. Anything else (service down, etc.) is worth surfacing.
        if (err instanceof ApiError && err.status !== 422) {
          setScanError(err.message);
        }
      } finally {
        checkingRef.current = false;
      }
    }, SCAN_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [employeeId, cameraError, result]);

  function scanAgain() {
    setResult(null);
    setScanError(null);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">Test Recognition</h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        <div className="relative aspect-square w-full overflow-hidden rounded-lg bg-slate-900">
          {cameraError && (
            <div className="absolute inset-0 z-10 flex items-center justify-center bg-slate-900 p-4 text-center text-sm text-slate-300">
              <WarningCircle size={18} className="mr-2 shrink-0" />
              {cameraError}
            </div>
          )}
          <video ref={videoRef} autoPlay playsInline muted className="h-full w-full object-cover" />

          {result?.matched && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-slate-900/80 text-center">
              <CheckCircle size={56} weight="fill" className="text-success" />
              <p className="text-lg font-bold text-white">{employeeName}</p>
              <p className="text-sm font-medium text-success">
                Verified &middot; {(result.confidence * 100).toFixed(1)}% confidence
              </p>
            </div>
          )}

          {!result && !cameraError && (
            <div className="absolute bottom-3 left-0 right-0 flex justify-center">
              <span className="flex items-center gap-2 rounded-full bg-slate-900/70 px-3 py-1.5 text-xs font-medium text-white">
                <span className="h-2 w-2 animate-pulse rounded-full bg-brand-blue" />
                Scanning for your face&hellip;
              </span>
            </div>
          )}
        </div>
        <canvas ref={canvasRef} className="hidden" />

        {scanError && <p className="mt-3 text-sm text-error">{scanError}</p>}

        <div className="mt-4 flex gap-2">
          {result ? (
            <>
              <button
                type="button"
                onClick={scanAgain}
                className="flex-1 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
              >
                Scan Again
              </button>
              <button
                type="button"
                onClick={onClose}
                className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                Done
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
