"use client";

import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  CheckCircle,
  ClockCounterClockwise,
  LockKey,
  ScanSmiley,
  SignIn,
  SignOut,
  WarningCircle,
  X,
} from "@phosphor-icons/react";
import { apiFetch, apiUpload, ApiError } from "@/lib/api";
import { captureFrameBlob, useCameraStream } from "@/lib/camera";
import { Avatar } from "@/components/Avatar";
import { StatCard } from "../_components/StatCard";
import { DayDetailModal } from "../_components/DayDetailModal";
import { EmployeeSearchPicker } from "../_components/EmployeeSearchPicker";
import { formatDate } from "../_components/my/types";
import { labelAttendanceEvents, type AttendanceEvent, type Employee, type OutReason } from "../employees/_components/types";
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

const SCAN_INTERVAL_MS = 1500;

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
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [todayEvents, setTodayEvents] = useState<AttendanceEvent[]>([]);
  const [snapshot, setSnapshot] = useState<TodaySnapshot | null>(null);
  const [autoOpen, setAutoOpen] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);
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

  useEffect(() => {
    if (new URLSearchParams(window.location.search).get("auto") === "1") {
      setAutoOpen(true);
    }
    apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
      .then((d) => setEmployees(d.employees ?? []))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employees"));
    apiFetch<TodaySnapshot>("/api/v1/dashboard/summary")
      .then(setSnapshot)
      .catch(() => {});
    loadToday();
  }, []);

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
            onClick={() => setAutoOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
          >
            <ScanSmiley size={16} />
            Add Attendance
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
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

      {autoOpen && (
        <AutoScanModal
          employees={employees}
          todayEvents={todayEvents}
          onClose={() => setAutoOpen(false)}
          onRecorded={() => loadToday()}
        />
      )}

      {manualOpen && (
        <ManualEntryModal
          employees={employees}
          onClose={() => setManualOpen(false)}
          onRecorded={() => {
            setManualOpen(false);
            loadToday();
          }}
        />
      )}
    </div>
  );
}

// Held between "the backend asked why" and "the person answered" — the same
// captured photo is resubmitted with the reason attached, so nobody has to
// scan twice.
type PendingReason = { employeeId: number; blob: Blob };

// The kiosk-style flow: no employee is picked ahead of time. Every frame is
// checked against everyone currently employed (1:N), and the moment someone
// is recognized, their next direction is recorded automatically. Marking OUT
// before their own shift ends pauses here to ask why before finishing.
function AutoScanModal({
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
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const cameraError = useCameraStream(videoRef);
  const checkingRef = useRef(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [result, setResult] = useState<AttendanceEvent | null>(null);
  const [liveStatus, setLiveStatus] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingReason | null>(null);

  useEffect(() => {
    if (cameraError || result || pending) return;

    const interval = setInterval(async () => {
      if (checkingRef.current) return;
      checkingRef.current = true;
      const blob = await captureFrameBlob(videoRef.current, canvasRef.current);
      if (!blob) {
        checkingRef.current = false;
        return;
      }
      try {
        const formData = new FormData();
        formData.append("file", blob, "scan.jpg");
        const event = await apiUpload<AttendanceEvent>("/api/v1/attendance/auto", formData);
        setScanError(null);
        setLiveStatus(null);
        setResult(event);
        onRecorded();
      } catch (err) {
        // The same photo that was already matched is held onto and
        // resubmitted with the reason once the person answers, rather than
        // capturing a fresh (possibly empty) frame.
        if (err instanceof ApiError && err.body?.reason_required && typeof err.body.employee_id === "number") {
          setPending({ employeeId: err.body.employee_id, blob });
          return;
        }
        // 422 covers "no face yet" and "not recognized" — both expected
        // while someone's getting into frame, so surface what the backend
        // actually saw instead of a static placeholder (e.g. "no face
        // detected" vs. "face not recognized" mean very different things).
        if (err instanceof ApiError) {
          if (err.status === 422) {
            setLiveStatus(err.message);
          } else {
            setScanError(err.message);
          }
        }
      } finally {
        checkingRef.current = false;
      }
    }, SCAN_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [cameraError, result, pending, onRecorded]);

  async function submitWithReason(reason: OutReason, note: string, documentFile?: File) {
    if (!pending) return;
    try {
      const formData = new FormData();
      formData.append("file", pending.blob, "scan.jpg");
      formData.append("reason", reason);
      if (note) formData.append("note", note);
      if (documentFile) formData.append("document", documentFile);
      const event = await apiUpload<AttendanceEvent>("/api/v1/attendance/auto", formData);
      setPending(null);
      setScanError(null);
      setResult(event);
      onRecorded();
    } catch (err) {
      setPending(null);
      setScanError(err instanceof ApiError ? err.message : "Failed to record attendance");
    }
  }

  const pendingEmployee = pending ? employees.find((e) => e.id === pending.employeeId) : null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">
            {pending ? "Why are you going out?" : "Add Attendance"}
          </h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        {/* The camera preview stays mounted for the whole life of this modal,
            pending or not — useCameraStream only ever binds the stream to
            the <video> element once, on this component's first mount, so
            conditionally unmounting it (e.g. swapping it out for the reason
            prompt) would leave a freshly-remounted <video> with no stream
            attached: a black box with no error, since the camera permission
            was already granted and the stream is still technically alive,
            just not attached to anything. Hidden with a CSS class instead
            while the reason prompt is up, never removed from the DOM. */}
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

            {!result && !cameraError && (
              <div className="absolute bottom-3 left-0 right-0 flex justify-center">
                <span className="flex items-center gap-2 rounded-full bg-slate-900/70 px-3 py-1.5 text-xs font-medium text-white">
                  <span className="h-2 w-2 animate-pulse rounded-full bg-brand-blue" />
                  {liveStatus ?? "Looking for a face…"}
                </span>
              </div>
            )}
          </div>
        </div>
        <canvas ref={canvasRef} className="hidden" />

        {pending ? (
          <OutReasonPrompt employeeName={pendingEmployee?.full_name} onSubmit={submitWithReason} onCancel={() => setPending(null)} />
        ) : (
          <>
            {scanError && <p className="mt-3 text-center text-sm text-error">{scanError}</p>}

            <div className="mt-4 flex gap-2">
              {result ? (
                <>
                  <button
                    type="button"
                    onClick={() => setResult(null)}
                    className="flex-1 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
                  >
                    Scan Next Person
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
          </>
        )}
      </div>
    </div>
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

  async function submit(withPassword?: string) {
    if (!selected) return;
    setSaving(true);
    setError(null);
    try {
      await apiFetch("/api/v1/attendance/manual", {
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
      onRecorded();
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
