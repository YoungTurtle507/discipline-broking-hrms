"use client";

import { Fragment, useCallback, useEffect, useState } from "react";
import { WarningCircle, PlusCircle, Flag, CircleNotch } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { type DailyAttendanceResult, type DailyAttendanceStatus } from "../../_components/types";
import { useSession } from "../../../_components/SessionProvider";

const STATUS_STYLES: Record<DailyAttendanceStatus, string> = {
  PRESENT: "bg-success-bg text-success",
  ABSENT: "bg-error-bg text-error",
  HALF_DAY: "bg-warning-bg text-warning",
  HOLIDAY: "bg-light-blue text-brand-blue",
  WEEKLY_OFF: "bg-light-blue text-brand-blue",
  ON_LEAVE: "bg-accent-purple-bg text-accent-purple",
  INACTIVE: "bg-slate-100 text-slate-400",
};

function formatMinutes(minutes: number): string {
  if (minutes === 0) return "—";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

// "Wed, 17/09/2026" — weekday for quick scanning, date itself in the app's
// standard DD/MM/YYYY order rather than a locale-dependent format.
function formatDate(iso: string): string {
  const weekday = new Date(iso + "T00:00:00").toLocaleDateString(undefined, { weekday: "short" });
  const [year, month, day] = iso.split("-");
  return `${weekday}, ${day}/${month}/${year}`;
}

// This duplicate's Attendance History is capped at 30 days (vs. the original's 14) — see the plan.
const HISTORY_DAYS = 30;

// Exceptions an admin can actually resolve by adding the scan the employee
// missed. Anything else is informational and has no correction to offer.
const FIXABLE = new Set(["MISSING_OUT", "MISSING_IN", "INVALID_SEQUENCE"]);

export function AttendanceHistory({ employeeId }: { employeeId: string }) {
  const { me } = useSession();
  const isSuperAdmin = me?.roles.includes("SUPER_ADMIN") ?? false;

  const [results, setResults] = useState<DailyAttendanceResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Adding a scan the employee forgot. The original events are never touched —
  // this appends the missing one and the day recalculates from the full set.
  const [fixing, setFixing] = useState<string | null>(null);
  const [fixTime, setFixTime] = useState("");
  const [fixDirection, setFixDirection] = useState<"IN" | "OUT">("OUT");
  const [fixNote, setFixNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [fixError, setFixError] = useState<string | null>(null);

  // Manually excusing an Uninformed Leave day (or reverting one already
  // excused) — Super Admin only. Treated identically to an approved leave
  // request everywhere the engine's output is consumed, not just this flag.
  const [overriding, setOverriding] = useState<string | null>(null);
  const [overrideAction, setOverrideAction] = useState<"excuse" | "unexcuse">("excuse");
  const [overrideNote, setOverrideNote] = useState("");
  const [overrideSaving, setOverrideSaving] = useState(false);
  const [overrideError, setOverrideError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const to = new Date();
    const from = new Date();
    from.setDate(from.getDate() - (HISTORY_DAYS - 1));
    const fmt = (d: Date) => d.toISOString().slice(0, 10);

    try {
      // Calculated live from the raw events rather than read from stored
      // results, which only exist for days somebody explicitly recalculated.
      const d = await apiFetch<{ results: DailyAttendanceResult[] | null }>(
        `/api/v1/attendance/calculated?employee_id=${employeeId}&from=${fmt(from)}&to=${fmt(to)}`
      );
      setResults((d.results ?? []).slice().reverse());
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load attendance history");
    }
  }, [employeeId]);

  useEffect(() => {
    load();
  }, [load]);

  function openFix(date: string, exceptions: string[]) {
    setFixing(date);
    setFixDirection(exceptions.includes("MISSING_IN") ? "IN" : "OUT");
    setFixTime("");
    setFixNote("");
    setFixError(null);
  }

  async function submitFix(e: React.FormEvent, date: string) {
    e.preventDefault();
    setFixError(null);
    setSaving(true);
    try {
      await apiFetch("/api/v1/attendance/manual", {
        method: "POST",
        body: JSON.stringify({
          employee_id: Number(employeeId),
          direction: fixDirection,
          event_time: `${date}T${fixTime}`,
          note: fixNote,
        }),
      });
      setFixing(null);
      await load();
    } catch (err) {
      setFixError(err instanceof ApiError ? err.message : "Failed to record the scan");
    } finally {
      setSaving(false);
    }
  }

  function openOverride(date: string, action: "excuse" | "unexcuse") {
    setOverriding(date);
    setOverrideAction(action);
    setOverrideNote("");
    setOverrideError(null);
  }

  async function submitOverride(e: React.FormEvent, date: string) {
    e.preventDefault();
    setOverrideError(null);
    setOverrideSaving(true);
    try {
      // The backend reads these as a form body (c.PostForm), not JSON.
      const body = new URLSearchParams({
        employee_id: employeeId,
        date,
        excuse: overrideAction === "excuse" ? "true" : "false",
        note: overrideNote,
      });
      await apiFetch("/api/v1/attendance/absence-override", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      setOverriding(null);
      await load();
    } catch (err) {
      setOverrideError(err instanceof ApiError ? err.message : "Failed to update the absence override");
    } finally {
      setOverrideSaving(false);
    }
  }

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
      <h2 className="mb-1 text-base font-semibold text-slate-900">Attendance History</h2>
      <p className="mb-4 text-xs text-slate-400">
        Last {HISTORY_DAYS} days &middot; calculated from raw check-in/out events against the assigned shift.
        A missed scan can be added; the original events are never changed.
      </p>

      {error && <p className="text-sm text-error">{error}</p>}

      {!error && !results && <p className="text-sm text-slate-400">Loading&hellip;</p>}

      {results && results.length === 0 && (
        <p className="text-sm text-slate-400">No completed days in this range yet.</p>
      )}

      {results && results.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] text-left text-sm">
            <thead className="text-xs font-semibold uppercase tracking-wide text-slate-400">
              <tr>
                <th className="py-2 pr-3">Date</th>
                <th className="py-2 pr-3">Status</th>
                <th className="py-2 pr-3">Hours</th>
                <th className="py-2 pr-3">Late</th>
                <th className="py-2 pr-3">Early Leave</th>
                <th className="py-2 pr-3">Overtime</th>
                <th className="py-2">Notes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {results.map((r) => {
                const fixable = (r.exceptions ?? []).some((e) => FIXABLE.has(e));
                return (
                <Fragment key={r.date}>
                <tr>
                  <td className="py-2.5 pr-3 text-slate-700">{formatDate(r.date)}</td>
                  <td className="py-2.5 pr-3">
                    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${STATUS_STYLES[r.status]}`}>
                      {r.status.replace("_", " ")}
                    </span>
                  </td>
                  <td className="py-2.5 pr-3 text-slate-600">{formatMinutes(r.working_minutes)}</td>
                  <td className="py-2.5 pr-3 text-slate-600">{formatMinutes(r.late_minutes)}</td>
                  <td className="py-2.5 pr-3 text-slate-600">{formatMinutes(r.early_leave_minutes)}</td>
                  <td className="py-2.5 pr-3 text-slate-600">{formatMinutes(r.overtime_minutes)}</td>
                  <td className="py-2.5">
                    <div className="flex items-center gap-2.5">
                      {(r.exceptions ?? []).length > 0 && (
                        <span
                          className="flex items-center gap-1.5 text-xs font-medium text-warning"
                          title={r.exceptions.join(", ")}
                        >
                          <WarningCircle size={14} weight="fill" />
                          {r.exceptions.length} flagged
                        </span>
                      )}
                      {fixable && (
                        <button
                          type="button"
                          onClick={() => (fixing === r.date ? setFixing(null) : openFix(r.date, r.exceptions))}
                          className="flex items-center gap-1 whitespace-nowrap text-xs font-semibold text-brand-blue hover:underline"
                        >
                          <PlusCircle size={13} weight="bold" />
                          Add missed scan
                        </button>
                      )}
                      {r.status === "ABSENT" && (
                        <span
                          className="flex items-center gap-1.5 text-xs font-semibold text-error"
                          title="No leave request or excuse is on file for this day — an unapproved-absence penalty applies"
                        >
                          <Flag size={13} weight="fill" />
                          Uninformed Leave
                        </span>
                      )}
                      {r.status === "ON_LEAVE" && r.is_excused_absence && (
                        <span
                          className="flex items-center gap-1.5 text-xs font-semibold text-accent-purple"
                          title="Manually excused by a Super Admin — no leave request is on file for this day"
                        >
                          <Flag size={13} weight="fill" />
                          Excused Absence
                        </span>
                      )}
                      {isSuperAdmin && r.status === "ABSENT" && (
                        <button
                          type="button"
                          onClick={() => (overriding === r.date ? setOverriding(null) : openOverride(r.date, "excuse"))}
                          className="flex items-center gap-1 whitespace-nowrap text-xs font-semibold text-brand-blue hover:underline"
                        >
                          Mark Informed
                        </button>
                      )}
                      {isSuperAdmin && r.status === "ON_LEAVE" && r.is_excused_absence && (
                        <button
                          type="button"
                          onClick={() => (overriding === r.date ? setOverriding(null) : openOverride(r.date, "unexcuse"))}
                          className="flex items-center gap-1 whitespace-nowrap text-xs font-semibold text-slate-500 hover:text-error hover:underline"
                        >
                          Undo
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
                {overriding === r.date && (
                  <tr className="bg-slate-50">
                    <td colSpan={7} className="px-2 py-3">
                      <form onSubmit={(e) => submitOverride(e, r.date)} className="flex flex-col gap-3 sm:flex-row sm:items-end">
                        <label className="block flex-1">
                          <span className="mb-1 block text-xs font-semibold text-slate-600">
                            {overrideAction === "excuse"
                              ? "Why was this absence informed?"
                              : "Why is this being reverted to Uninformed Leave?"}
                          </span>
                          <input
                            value={overrideNote}
                            onChange={(e) => setOverrideNote(e.target.value)}
                            placeholder={
                              overrideAction === "excuse"
                                ? "e.g. called in sick, confirmed with their manager"
                                : "optional"
                            }
                            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-brand-blue"
                          />
                        </label>
                        <div className="flex gap-2">
                          <button
                            type="submit"
                            disabled={overrideSaving}
                            className="flex items-center gap-2 rounded-xl bg-navy px-4 py-2 text-sm font-semibold text-white hover:bg-navy-light disabled:opacity-70"
                          >
                            {overrideSaving && <CircleNotch size={14} className="animate-spin" />}
                            {overrideAction === "excuse" ? "Mark Informed" : "Revert to Uninformed"}
                          </button>
                          <button
                            type="button"
                            onClick={() => setOverriding(null)}
                            className="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100"
                          >
                            Cancel
                          </button>
                        </div>
                        <p className="basis-full text-xs text-slate-500">
                          {overrideAction === "excuse"
                            ? "Treated the same as an approved leave request everywhere — the leave ledger, Reports, attendance %. Super Admin only."
                            : "Restores the unapproved-absence penalty for this day. Super Admin only."}
                        </p>
                        {overrideError && <p className="basis-full text-xs text-error">{overrideError}</p>}
                      </form>
                    </td>
                  </tr>
                )}
                {fixing === r.date && (
                  <tr className="bg-slate-50">
                    <td colSpan={7} className="px-2 py-3">
                      <form onSubmit={(e) => submitFix(e, r.date)} className="flex flex-col gap-3 sm:flex-row sm:items-end">
                        <label className="block">
                          <span className="mb-1 block text-xs font-semibold text-slate-600">Scan</span>
                          <select
                            value={fixDirection}
                            onChange={(e) => setFixDirection(e.target.value as "IN" | "OUT")}
                            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-brand-blue"
                          >
                            <option value="OUT">Check-out</option>
                            <option value="IN">Check-in</option>
                          </select>
                        </label>
                        <label className="block">
                          <span className="mb-1 block text-xs font-semibold text-slate-600">
                            Time on {formatDate(r.date)}
                          </span>
                          <input
                            type="time"
                            value={fixTime}
                            onChange={(e) => setFixTime(e.target.value)}
                            required
                            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-brand-blue"
                          />
                        </label>
                        <label className="block flex-1">
                          <span className="mb-1 block text-xs font-semibold text-slate-600">
                            Reason <span className="text-error">*</span>
                          </span>
                          <input
                            value={fixNote}
                            onChange={(e) => setFixNote(e.target.value)}
                            required
                            placeholder="e.g. forgot to scan out, confirmed with their manager"
                            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-brand-blue"
                          />
                        </label>
                        <div className="flex gap-2">
                          <button
                            type="submit"
                            disabled={saving}
                            className="flex items-center gap-2 rounded-xl bg-navy px-4 py-2 text-sm font-semibold text-white hover:bg-navy-light disabled:opacity-70"
                          >
                            {saving && <CircleNotch size={14} className="animate-spin" />}
                            Add scan
                          </button>
                          <button
                            type="button"
                            onClick={() => setFixing(null)}
                            className="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100"
                          >
                            Cancel
                          </button>
                        </div>
                        <p className="basis-full text-xs text-slate-500">
                          Recorded as a manual entry against your name, alongside the original scans. The day
                          recalculates immediately.
                        </p>
                        {fixError && <p className="basis-full text-xs text-error">{fixError}</p>}
                      </form>
                    </td>
                  </tr>
                )}
                </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
