"use client";

import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  Camera,
  CheckCircle,
  ClockCounterClockwise,
  HourglassMedium,
  LockKey,
  ScanSmiley,
  SignIn,
  SignOut,
  WarningCircle,
  XCircle,
  X,
} from "@phosphor-icons/react";
import { apiFetch, apiUpload, ApiError } from "@/lib/api";
import { captureFrameBlob, useCameraStream } from "@/lib/camera";
import { Avatar } from "@/components/Avatar";
import { StatCard } from "../_components/StatCard";
import { DayDetailModal } from "../_components/DayDetailModal";
import { EmployeeSearchPicker } from "../_components/EmployeeSearchPicker";
import { formatDate } from "../_components/my/types";
import { labelAttendanceEvents, type AttendanceEvent, type Employee, type ManualRequest, type OutReason } from "../employees/_components/types";
import { useSession } from "../_components/SessionProvider";

const OUT_REASON_LABEL: Record<OutReason, string> = {
  HALF_DAY: "Half Day",
  LUNCH: "Lunch",
  PERSONAL: "Personal",
  EARLY_LEAVE: "Early Leave",
  LATE_COMING: "Late Coming",
};

// Half Day, Early Leave and Personal need an explanation on record; Lunch is
// self-explanatory and Late Coming is never picked alongside a note field.
const NOTE_REQUIRED_REASONS = new Set<OutReason>(["HALF_DAY", "EARLY_LEAVE", "PERSONAL"]);

// The reason options offered on a manual correction, per direction — mirrors
// attendance.ValidManualReason on the backend.
const MANUAL_IN_REASONS: OutReason[] = ["LATE_COMING", "HALF_DAY", "LUNCH", "PERSONAL"];
const MANUAL_OUT_REASONS: OutReason[] = ["EARLY_LEAVE", "HALF_DAY", "LUNCH", "PERSONAL"];

// A <input type="datetime-local"> reads its value as local wall-clock time,
// so building this from toISOString() (which converts to UTC first) would
// silently cap the picker several hours into the past in any timezone ahead
// of UTC.
function localDateTimeStr(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

type TodaySnapshot = {
  at_work: number;
  absent: number;
  late: number;
  on_leave: number;
};

export default function AttendancePage() {
  const { me } = useSession();
  const isSuperAdmin = me?.roles.includes("SUPER_ADMIN") ?? false;

  const [employees, setEmployees] = useState<Employee[]>([]);
  const [todayEvents, setTodayEvents] = useState<AttendanceEvent[]>([]);
  const [snapshot, setSnapshot] = useState<TodaySnapshot | null>(null);
  const [markOpen, setMarkOpen] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);
  const [employeeFilter, setEmployeeFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [drill, setDrill] = useState<{
    title: string;
    status?: string;
    lateOnly?: boolean;
  } | null>(null);

  async function loadToday() {
    try {
      const data = await apiFetch<{ events: AttendanceEvent[] | null }>("/api/v1/attendance/today");
      setTodayEvents(data.events ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load today's attendance");
    }
  }

  async function loadPendingCount() {
    if (!isSuperAdmin) return;
    try {
      const data = await apiFetch<{ requests: ManualRequest[] | null }>("/api/v1/attendance/manual-requests");
      setPendingCount((data.requests ?? []).length);
    } catch {
      // Non-critical — the button just shows no badge if this fails.
    }
  }

  useEffect(() => {
    if (new URLSearchParams(window.location.search).get("auto") === "1") {
      setMarkOpen(true);
    }
    apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
      .then((d) => setEmployees(d.employees ?? []))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employees"));
    apiFetch<TodaySnapshot>("/api/v1/dashboard/summary")
      .then(setSnapshot)
      .catch(() => {});
    loadToday();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadPendingCount();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSuperAdmin]);

  const filteredEvents = employeeFilter
    ? todayEvents.filter((ev) => String(ev.employee_id) === employeeFilter)
    : todayEvents;
  const employeeFilterSelected = employees.find((e) => String(e.id) === employeeFilter) ?? null;
  const eventLabels = labelAttendanceEvents(todayEvents);

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Attendance</h1>
          <p className="mt-1 text-sm text-slate-500">Live face-scan attendance and today&apos;s activity.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {isSuperAdmin && (
            <button
              type="button"
              onClick={() => setRequestsOpen(true)}
              className="relative flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50"
            >
              <HourglassMedium size={16} />
              Pending Requests
              {pendingCount > 0 && (
                <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-warning px-1.5 text-xs font-bold text-white">
                  {pendingCount}
                </span>
              )}
            </button>
          )}
          <button
            type="button"
            onClick={() => setManualOpen(true)}
            className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50"
          >
            <ClockCounterClockwise size={16} />
            Correct a Missed Scan
          </button>
          <button
            type="button"
            onClick={() => setMarkOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
          >
            <ScanSmiley size={16} />
            Mark Attendance
          </button>
        </div>
      </div>

      {error && <p className="mb-4 text-sm text-error">{error}</p>}

      <div className="mb-6 flex flex-wrap rounded-xl border border-slate-200 bg-white shadow-card">
        <StatCard
          label="Present Today"
          tone="success"
          value={snapshot?.at_work ?? "—"}
          sub="full + half day"
          onClick={() => setDrill({ title: "Present Today", status: "PRESENT,HALF_DAY" })}
        />
        <StatCard
          label="Absent Today"
          tone="error"
          value={snapshot?.absent ?? "—"}
          sub="no approved leave"
          onClick={() => setDrill({ title: "Absent Today", status: "ABSENT" })}
        />
        <StatCard
          label="Late Coming"
          tone="warning"
          value={snapshot?.late ?? "—"}
          sub="past grace period"
          onClick={() => setDrill({ title: "Late Coming Today", lateOnly: true })}
        />
        <StatCard
          label="On Leave"
          tone="accent"
          value={snapshot?.on_leave ?? "—"}
          sub="approved today"
          onClick={() => setDrill({ title: "On Leave Today", status: "ON_LEAVE" })}
        />
      </div>

      {drill && (
        <DayDetailModal
          title={drill.title}
          status={drill.status}
          lateOnly={drill.lateOnly}
          onClose={() => setDrill(null)}
        />
      )}

      <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-base font-semibold text-slate-900">Today&apos;s Attendance</h2>
          <div className="w-full max-w-[260px]">
            <EmployeeSearchPicker
              employees={employees}
              selected={employeeFilterSelected}
              onSelect={(emp) => setEmployeeFilter(emp ? String(emp.id) : "")}
              placeholder="Filter by employee…"
              changeLabel="Clear"
            />
          </div>
        </div>
        {todayEvents.length === 0 ? (
          <p className="text-sm text-slate-400">No attendance recorded yet today.</p>
        ) : filteredEvents.length === 0 ? (
          <p className="text-sm text-slate-400">No IN/OUT recorded for this employee today.</p>
        ) : (
          <div className="max-h-[420px] overflow-y-auto pr-1">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {filteredEvents.map((ev) => (
                <div key={ev.id} className="flex items-center gap-3 rounded-lg border border-slate-100 px-3 py-2.5">
                  <Avatar name={ev.employee_name} employeeId={ev.employee_id} size={32} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-slate-800">{ev.employee_name}</p>
                    <p className="truncate text-xs text-slate-400">
                      {new Date(ev.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                      {ev.out_reason && (
                        <span className="text-accent-purple">
                          {" "}
                          ({OUT_REASON_LABEL[ev.out_reason]}
                          {ev.note ? `: ${ev.note}` : ""})
                        </span>
                      )}
                    </p>
                  </div>
                  <span
                    className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${
                      ev.direction === "IN" ? "bg-success-bg text-success" : "bg-warning-bg text-warning"
                    }`}
                  >
                    {eventLabels.get(ev.id) ?? ev.direction}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {markOpen && (
        <MarkAttendanceModal
          employees={employees}
          todayEvents={todayEvents}
          onClose={() => setMarkOpen(false)}
          onRecorded={() => loadToday()}
        />
      )}

      {manualOpen && (
        <ManualEntryModal
          employees={employees}
          onClose={() => {
            setManualOpen(false);
            // Covers the kiosk's "submitted, pending approval" path too —
            // that one never calls onRecorded (nothing landed in today's
            // feed yet), but the pending badge still needs to pick it up.
            loadPendingCount();
          }}
          onRecorded={() => {
            setManualOpen(false);
            loadToday();
          }}
        />
      )}

      {requestsOpen && (
        <ManualRequestsModal
          onClose={() => setRequestsOpen(false)}
          onReviewed={() => {
            loadPendingCount();
            loadToday();
          }}
        />
      )}
    </div>
  );
}

// Held once the backend asks why someone's going out — the same captured
// photo is resubmitted with the reason attached, so nobody has to capture
// twice.
type PendingReason = { blob: Blob };

// "Mark Attendance": the employee is picked first (step "pick"), then a
// camera screen appears for a single manual capture (step "capture") — no
// auto-detection, no 1:N face match. The captured photo only has to show one
// real, live person; the backend stamps it with date/direction/time and
// stores it against the event it creates.
function MarkAttendanceModal({
  employees,
  todayEvents,
  onClose,
  onRecorded,
}: {
  employees: Employee[];
  todayEvents: AttendanceEvent[];
  onClose: () => void;
  onRecorded: () => void;
}) {
  const [step, setStep] = useState<"pick" | "capture">("pick");
  const [employee, setEmployee] = useState<Employee | null>(null);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">
            {step === "pick" ? "Mark Attendance" : employee?.full_name}
          </h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        {step === "pick" ? (
          <>
            <p className="mb-3 text-sm text-slate-500">Select your name to continue.</p>
            <EmployeeSearchPicker
              employees={employees}
              selected={null}
              onSelect={(emp) => {
                if (!emp) return;
                setEmployee(emp);
                setStep("capture");
              }}
              placeholder="Search your name…"
              autoFocus
            />
            <button
              type="button"
              onClick={onClose}
              className="mt-4 w-full rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
          </>
        ) : (
          <CaptureScreen
            employee={employee!}
            todayEvents={todayEvents}
            onBack={() => setStep("pick")}
            onClose={onClose}
            onRecorded={onRecorded}
          />
        )}
      </div>
    </div>
  );
}

// The camera step for one already-chosen employee: live preview, a manual
// Capture button (no auto-polling — a capture only happens when clicked),
// then either a result or (marking OUT before shift end) the reason prompt.
// When the face service last proved itself awake (a warm-up or a successful
// scan). Render's free tier sleeps it after ~15 idle minutes, so for 10 minutes
// after this the capture screen skips the wake-up and its message.
const FACE_WARM_WINDOW_MS = 10 * 60 * 1000;
let faceWarmAt = 0;
const faceRecentlyWarm = () => Date.now() - faceWarmAt < FACE_WARM_WINDOW_MS;

function CaptureScreen({
  employee,
  todayEvents,
  onBack,
  onClose,
  onRecorded,
}: {
  employee: Employee;
  todayEvents: AttendanceEvent[];
  onBack: () => void;
  onClose: () => void;
  onRecorded: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const cameraError = useCameraStream(videoRef);
  const [submitting, setSubmitting] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [result, setResult] = useState<AttendanceEvent | null>(null);
  const [pending, setPending] = useState<PendingReason | null>(null);
  // The face service sleeps when idle on free hosting; ask the backend to wake
  // it as soon as this screen opens so it's ready by the time Capture is hit.
  const [faceWarm, setFaceWarm] = useState<"warming" | "ready" | "failed">(
    () => (faceRecentlyWarm() ? "ready" : "warming"),
  );

  useEffect(() => {
    if (faceRecentlyWarm()) return;
    let cancelled = false;
    apiFetch("/api/v1/attendance/face-warmup")
      .then(() => {
        faceWarmAt = Date.now();
        if (!cancelled) setFaceWarm("ready");
      })
      .catch(() => !cancelled && setFaceWarm("failed"));
    return () => {
      cancelled = true;
    };
  }, []);

  async function submit(blob: Blob, reason?: OutReason, note?: string, documentFile?: File) {
    setSubmitting(true);
    try {
      const formData = new FormData();
      formData.append("employee_id", String(employee.id));
      formData.append("file", blob, "capture.jpg");
      if (reason) formData.append("reason", reason);
      if (note) formData.append("note", note);
      if (documentFile) formData.append("document", documentFile);
      const event = await apiUpload<AttendanceEvent>("/api/v1/attendance", formData);
      setPending(null);
      setScanError(null);
      faceWarmAt = Date.now();
      setResult(event);
      onRecorded();
    } catch (err) {
      // The same photo already submitted is held onto and resubmitted with
      // the reason once the person answers, rather than capturing a fresh one.
      if (!reason && err instanceof ApiError && err.body?.reason_required) {
        setPending({ blob });
        return;
      }
      setPending(null);
      setScanError(err instanceof ApiError ? err.message : "Failed to record attendance");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCapture() {
    const blob = await captureFrameBlob(videoRef.current, canvasRef.current);
    if (!blob) return;
    await submit(blob);
  }

  function submitWithReason(reason: OutReason, note: string, documentFile?: File) {
    if (!pending) return;
    void submit(pending.blob, reason, note, documentFile);
  }

  return (
    <>
      {/* The camera preview stays mounted for this whole step, pending or
          not — useCameraStream only ever binds the stream to the <video>
          element once, on first mount, so conditionally unmounting it (e.g.
          swapping it out for the reason prompt) would leave a freshly
          remounted <video> with no stream attached. Hidden with a CSS class
          instead while the reason prompt is up, never removed from the DOM. */}
      <div className={pending ? "hidden" : undefined}>
        <div className="relative aspect-square w-full overflow-hidden rounded-lg bg-slate-900">
          {cameraError && (
            <div className="absolute inset-0 z-10 flex items-center justify-center bg-slate-900 p-4 text-center text-sm text-slate-300">
              <WarningCircle size={18} className="mr-2 shrink-0" />
              {cameraError}
            </div>
          )}
          <video ref={videoRef} autoPlay playsInline muted className="h-full w-full object-cover" />

          {result && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-slate-900/85 text-center">
              {result.direction === "IN" ? (
                <SignIn size={48} weight="bold" className="text-success" />
              ) : (
                <SignOut size={48} weight="bold" className="text-warning" />
              )}
              <p className="text-lg font-bold text-white">{result.employee_name}</p>
              <p className="text-sm font-medium text-slate-300">
                {labelAttendanceEvents([...todayEvents, result]).get(result.id) ?? result.direction} Recorded
                {result.out_reason ? ` (${OUT_REASON_LABEL[result.out_reason]})` : ""} &middot;{" "}
                {new Date(result.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
              </p>
            </div>
          )}
        </div>
      </div>
      <canvas ref={canvasRef} className="hidden" />

      {pending ? (
        <OutReasonPrompt employeeName={employee.full_name} onSubmit={submitWithReason} onCancel={() => setPending(null)} />
      ) : (
        <>
          {faceWarm === "warming" && !result && (
            <p className="mt-3 text-center text-sm text-slate-500">Waking up face service… you can capture now, it may take a little longer.</p>
          )}
          {scanError && <p className="mt-3 text-center text-sm text-error">{scanError}</p>}

          <div className="mt-4 flex gap-2">
            {result ? (
              <>
                <button
                  type="button"
                  onClick={onBack}
                  className="flex-1 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
                >
                  Mark Another Person
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
              <>
                <button
                  type="button"
                  onClick={onBack}
                  className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
                >
                  Back
                </button>
                <button
                  type="button"
                  disabled={!!cameraError || submitting}
                  onClick={handleCapture}
                  className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Camera size={16} weight="bold" />
                  {submitting ? (faceWarm === "ready" ? "Capturing…" : "Capturing… (face service waking, up to a minute)") : "Capture"}
                </button>
              </>
            )}
          </div>
        </>
      )}
    </>
  );
}

// Shown in place of the camera once a mid-day OUT scan is recognized: pick
// Half Day (leaving for the rest of the day), Early Leave, Lunch, or Personal
// (the latter two both meaning back later today), with an optional note,
// before the entry is finalized.
function OutReasonPrompt({
  employeeName,
  onSubmit,
  onCancel,
}: {
  employeeName?: string;
  onSubmit: (reason: OutReason, note: string, documentFile?: File) => void;
  onCancel: () => void;
}) {
  const [reason, setReason] = useState<OutReason | null>(null);
  const [note, setNote] = useState("");
  const [documentFile, setDocumentFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);

  return (
    <div>
      {employeeName && <p className="mb-4 text-sm text-slate-600">{employeeName} is marking out before their shift ends.</p>}
      <div className="flex flex-col gap-2">
        {(["HALF_DAY", "EARLY_LEAVE", "LUNCH", "PERSONAL"] as const).map((r) => (
          <button
            key={r}
            type="button"
            onClick={() => setReason(r)}
            className={`rounded-lg border px-4 py-3 text-left text-sm font-semibold transition-colors ${
              reason === r ? "border-brand-blue bg-light-blue text-brand-blue" : "border-slate-200 text-slate-700 hover:bg-slate-50"
            }`}
          >
            {OUT_REASON_LABEL[r]}
            <span className="mt-0.5 block text-xs font-normal text-slate-400">
              {r === "HALF_DAY" && "Taking this as a half day, regardless of hours worked"}
              {r === "EARLY_LEAVE" && "Leaving for the day, but it still counts normally for the time worked"}
              {r === "LUNCH" && "Lunch break — back today"}
              {r === "PERSONAL" && "Personal errand — back today"}
            </span>
          </button>
        ))}
      </div>

      <div className="mt-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">
          Note {reason && NOTE_REQUIRED_REASONS.has(reason) ? <span className="text-red-500">(required)</span> : "(optional)"}
        </label>
        <input
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder="e.g. Doctor's appointment"
          className="input"
        />
      </div>

      {reason === "HALF_DAY" && (
        <div className="mt-3">
          <label className="mb-1 block text-xs font-semibold text-slate-600">
            Supporting document <span className="font-normal text-slate-400">(optional)</span>
          </label>
          <input
            type="file"
            accept=".pdf,.jpg,.jpeg,.png,.webp"
            onChange={(e) => setDocumentFile(e.target.files?.[0] ?? null)}
            className="block w-full text-xs text-slate-500 file:mr-2 file:rounded-lg file:border-0 file:bg-light-blue file:px-3 file:py-1.5 file:text-xs file:font-semibold file:text-brand-blue"
          />
          <p className="mt-1 text-xs text-slate-400">
            This half day will show in Leave Management either way — attach proof now or add it there later.
          </p>
        </div>
      )}

      <div className="mt-4 flex gap-2">
        <button
          type="button"
          disabled={!reason || submitting || (reason != null && NOTE_REQUIRED_REASONS.has(reason) && note.trim() === "")}
          onClick={() => {
            if (!reason) return;
            setSubmitting(true);
            onSubmit(reason, note, documentFile ?? undefined);
          }}
          className="flex-1 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
        >
          {submitting ? "Recording…" : "Confirm"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// A scan the employee forgot — almost always a missed check-out. This adds an
// event rather than editing one, matching the backend's own rule that a
// correction is additive so the original (incomplete) record survives.
function ManualEntryModal({
  employees,
  onClose,
  onRecorded,
}: {
  employees: Employee[];
  onClose: () => void;
  onRecorded: () => void;
}) {
  const { me } = useSession();
  const isSuperAdmin = me?.roles.includes("SUPER_ADMIN") ?? false;

  const [selected, setSelected] = useState<Employee | null>(null);
  const [direction, setDirection] = useState<"IN" | "OUT">("OUT");
  const [eventTime, setEventTime] = useState("");
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Applies the same rules a live scan's reason popup would — Late Coming or
  // Half Day when correcting an IN, Early Leave/Half Day/Lunch when
  // correcting an OUT. Optional; cleared whenever the direction changes since
  // the two option sets don't overlap.
  const [outReason, setOutReason] = useState<OutReason | "">("");
  const reasonOptions = direction === "IN" ? MANUAL_IN_REASONS : MANUAL_OUT_REASONS;

  // A Super Admin already holds every permission unconditionally, so this
  // step is skipped for them — anyone else confirms their own login password
  // before the entry actually gets recorded.
  const [stage, setStage] = useState<"form" | "password">("form");
  const [password, setPassword] = useState("");
  // Set once a non-Super-Admin's submission comes back pending approval
  // instead of recorded — see Handlers.RecordManual's branch on
  // IsSuperAdmin(). Nothing was actually recorded yet, so this shows its
  // own confirmation rather than calling onRecorded (which refreshes
  // today's feed — there's nothing new in it yet).
  const [submittedPending, setSubmittedPending] = useState(false);

  async function submit(withPassword?: string) {
    if (!selected) return;
    setSaving(true);
    setError(null);
    try {
      const result = await apiFetch<{ pending_approval?: boolean }>("/api/v1/attendance/manual", {
        method: "POST",
        body: JSON.stringify({
          employee_id: selected.id,
          direction,
          event_time: eventTime,
          note,
          out_reason: outReason || undefined,
          ...(withPassword ? { password: withPassword } : {}),
        }),
      });
      if (result.pending_approval) {
        setSubmittedPending(true);
      } else {
        onRecorded();
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to record the entry");
    } finally {
      setSaving(false);
    }
  }

  function handleFormSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!selected) return;
    if (isSuperAdmin) {
      submit();
    } else {
      setError(null);
      setStage("password");
    }
  }

  function handlePasswordSubmit(e: React.FormEvent) {
    e.preventDefault();
    submit(password);
  }

  if (submittedPending) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
        <div className="w-full max-w-md rounded-xl bg-white p-6 text-center shadow-xl">
          <CheckCircle size={40} weight="fill" className="mx-auto mb-3 text-brand-blue" />
          <h2 className="mb-1 text-base font-semibold text-slate-900">Request Submitted</h2>
          <p className="mb-5 text-sm text-slate-500">
            Waiting on Super Admin approval for {selected?.full_name}&apos;s entry — it won&apos;t appear in attendance until then.
          </p>
          <button
            type="button"
            onClick={onClose}
            className="w-full rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
          >
            Done
          </button>
        </div>
      </div>
    );
  }

  if (stage === "password") {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
        <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-base font-semibold text-slate-900">Confirm Your Password</h2>
            <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
              <X size={20} />
            </button>
          </div>

          <form onSubmit={handlePasswordSubmit} className="flex flex-col gap-4">
            <p className="text-sm text-slate-600">
              Enter your login password to record this entry for {selected?.full_name}.
            </p>
            <div>
              <label className="mb-1 block text-xs font-semibold text-slate-600">Password</label>
              <div className="relative">
                <LockKey size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoFocus
                  autoComplete="current-password"
                  className="input pl-9"
                />
              </div>
            </div>

            {error && <p className="text-sm text-error">{error}</p>}

            <div className="flex gap-2">
              <button
                type="submit"
                disabled={saving || !password}
                className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
              >
                <CheckCircle size={16} weight="bold" />
                {saving ? "Recording…" : "Record Entry"}
              </button>
              <button
                type="button"
                onClick={() => {
                  setStage("form");
                  setPassword("");
                  setError(null);
                }}
                className="flex items-center gap-1.5 rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                <ArrowLeft size={15} />
                Back
              </button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">Correct a Missed Scan</h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleFormSubmit} className="flex flex-col gap-4">
          {!selected && <label className="mb-1 block text-xs font-semibold text-slate-600">Employee</label>}
          <EmployeeSearchPicker employees={employees} selected={selected} onSelect={setSelected} autoFocus />

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">Direction</label>
            <div className="flex gap-2">
              {(["IN", "OUT"] as const).map((d) => (
                <button
                  key={d}
                  type="button"
                  onClick={() => {
                    setDirection(d);
                    setOutReason("");
                  }}
                  className={`flex-1 rounded-lg border px-3 py-2 text-sm font-semibold transition-colors ${
                    direction === d
                      ? "border-brand-blue bg-light-blue text-brand-blue"
                      : "border-slate-200 text-slate-600 hover:bg-slate-50"
                  }`}
                >
                  {d}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">
              Reason <span className="font-normal text-slate-400">(optional)</span>
            </label>
            <div className="flex flex-wrap gap-2">
              {reasonOptions.map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setOutReason(outReason === r ? "" : r)}
                  className={`rounded-lg border px-3 py-2 text-sm font-semibold transition-colors ${
                    outReason === r
                      ? "border-brand-blue bg-light-blue text-brand-blue"
                      : "border-slate-200 text-slate-600 hover:bg-slate-50"
                  }`}
                >
                  {OUT_REASON_LABEL[r]}
                </button>
              ))}
            </div>
            <span className="mt-1 block text-xs text-slate-400">
              {direction === "IN"
                ? "Applies the same calculation a live scan's reason would — e.g. force this arrival to count as late."
                : "Applies the same calculation the kiosk's reason popup would — e.g. force this day to Half Day."}
            </span>
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">Date &amp; Time</label>
            <input
              type="datetime-local"
              value={eventTime}
              onChange={(e) => setEventTime(e.target.value)}
              max={localDateTimeStr(new Date())}
              required
              className="input"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">Reason</label>
            <input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="e.g. Forgot to scan out at end of shift"
              required
              className="input"
            />
          </div>

          {error && <p className="text-sm text-error">{error}</p>}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={!selected || saving}
              className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
            >
              <CheckCircle size={16} weight="bold" />
              {saving ? "Saving…" : isSuperAdmin ? "Record Entry" : "Continue"}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Super Admin's review queue for the kiosk login's missed-scan corrections —
// each one only becomes a real attendance event once approved here (see
// Handlers.ApproveManualRequest, which re-checks the IN/OUT alternation
// rule against whatever's on record *now*, not just at submission time).
function ManualRequestsModal({ onClose, onReviewed }: { onClose: () => void; onReviewed: () => void }) {
  const [requests, setRequests] = useState<ManualRequest[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actingOn, setActingOn] = useState<number | null>(null);

  async function load() {
    try {
      const data = await apiFetch<{ requests: ManualRequest[] | null }>("/api/v1/attendance/manual-requests");
      setRequests(data.requests ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load requests");
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function act(id: number, action: "approve" | "reject") {
    setActingOn(id);
    setError(null);
    try {
      await apiFetch(`/api/v1/attendance/manual-requests/${id}/${action}`, { method: "POST" });
      setRequests((prev) => (prev ?? []).filter((r) => r.id !== id));
      onReviewed();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : `Failed to ${action} the request`);
    } finally {
      setActingOn(null);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">Pending Requests</h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        {error && <p className="mb-3 text-sm text-error">{error}</p>}

        {requests === null ? (
          <p className="py-6 text-center text-sm text-slate-400">Loading…</p>
        ) : requests.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-400">No pending requests.</p>
        ) : (
          <div className="flex flex-col gap-3">
            {requests.map((r) => (
              <div key={r.id} className="rounded-lg border border-slate-200 p-4">
                <div className="mb-2 flex items-start justify-between gap-2">
                  <div>
                    <p className="text-sm font-semibold text-slate-800">{r.employee_name}</p>
                    <p className="text-xs text-slate-400">
                      {r.direction} &middot; {formatDate(r.event_time)} &middot;{" "}
                      {new Date(r.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                      {r.out_reason ? ` (${OUT_REASON_LABEL[r.out_reason]})` : ""}
                    </p>
                  </div>
                  <span
                    className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${
                      r.direction === "IN" ? "bg-success-bg text-success" : "bg-warning-bg text-warning"
                    }`}
                  >
                    {r.direction}
                  </span>
                </div>
                <p className="mb-3 text-sm text-slate-600">{r.note}</p>
                <p className="mb-3 text-xs text-slate-400">
                  Requested by {r.requested_by_name} &middot; {new Date(r.created_at).toLocaleString([], { dateStyle: "medium", timeStyle: "short" })}
                </p>
                <div className="flex gap-2">
                  <button
                    type="button"
                    disabled={actingOn === r.id}
                    onClick={() => act(r.id, "approve")}
                    className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-success px-3 py-2 text-sm font-semibold text-white hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <CheckCircle size={15} weight="bold" />
                    Approve
                  </button>
                  <button
                    type="button"
                    disabled={actingOn === r.id}
                    onClick={() => act(r.id, "reject")}
                    className="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <XCircle size={15} weight="bold" />
                    Reject
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
