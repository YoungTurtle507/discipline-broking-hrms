"use client";

import { useEffect, useState } from "react";
import { X, WarningCircle } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { Avatar } from "@/components/Avatar";
import { formatClock } from "./my/types";

type DayDetailEntry = {
  employee_id: number;
  employee_code: string;
  full_name: string;
  department_name: string | null;
  designation_title: string | null;
  status: string;
  first_in: string | null;
  last_out: string | null;
  working_minutes: number;
  late_minutes: number;
  early_leave_minutes: number;
};

// "arrival" shows when they came in (the default, used everywhere except the
// Early Leave card); "departure" shows when they left instead; "both" shows
// the full Office In – Office Out span, used only for the Half Day drill-down
// where knowing when they left matters as much as when they arrived.
function EntryRow({ e, mode = "arrival" }: { e: DayDetailEntry; mode?: "arrival" | "departure" | "both" }) {
  if (mode === "both") {
    return (
      <li className="flex items-center gap-3 px-5 py-3">
        <Avatar name={e.full_name} employeeId={e.employee_id} size={34} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-slate-800">{e.full_name}</p>
          <p className="truncate text-xs text-slate-400">
            {e.employee_code}
            {e.designation_title ? ` · ${e.designation_title}` : ""}
          </p>
        </div>
        <div className="shrink-0 text-right text-xs text-slate-500">
          <span className="block font-mono">
            {e.first_in ? formatClock(e.first_in) : "—"} – {e.last_out ? formatClock(e.last_out) : "—"}
          </span>
          {e.late_minutes > 0 && <span className="block text-warning">{e.late_minutes}m late</span>}
        </div>
      </li>
    );
  }

  const clockValue = mode === "departure" ? e.last_out : e.first_in;
  return (
    <li className="flex items-center gap-3 px-5 py-3">
      <Avatar name={e.full_name} employeeId={e.employee_id} size={34} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-slate-800">{e.full_name}</p>
        <p className="truncate text-xs text-slate-400">
          {e.employee_code}
          {e.designation_title ? ` · ${e.designation_title}` : ""}
        </p>
      </div>
      <div className="shrink-0 text-right text-xs text-slate-500">
        {clockValue ? (
          <>
            <span className="block font-mono">{formatClock(clockValue)}</span>
            {mode === "arrival" && e.late_minutes > 0 && (
              <span className="block text-warning">{e.late_minutes}m late</span>
            )}
          </>
        ) : (
          <span className="text-slate-300">—</span>
        )}
      </div>
    </li>
  );
}

// The drill-down behind a dashboard stat card: no date is passed — the
// backend defaults to its own "today", the same one the summary card's count
// came from, so the two can never disagree over a client/server timezone gap.
export function DayDetailModal({
  title,
  status,
  highlightLate,
  earlyLeaveOnly,
  lateOnly,
  showBothTimes,
  onClose,
}: {
  title: string;
  status?: string;
  // Splits the list into its own "Arrived Late" section up top, followed by
  // everyone else — used by the Present card, where "who's late" is the
  // thing HR actually opens this to check.
  highlightLate?: boolean;
  // Shows Office In – Office Out instead of a single time — used by the Half
  // Day drill-down, where knowing when they left matters as much as arrival.
  showBothTimes?: boolean;
  // Early leave isn't a status of its own — Present or Half Day can both
  // carry it — so this ignores the status filter entirely and instead keeps
  // only whoever actually left early, showing when they left.
  earlyLeaveOnly?: boolean;
  // Same idea for the Attendance page's own Late Coming card: ignores the
  // status filter and keeps only whoever actually arrived late, in one flat
  // list rather than split into "late" + "on time" sections.
  lateOnly?: boolean;
  onClose: () => void;
}) {
  const [entries, setEntries] = useState<DayDetailEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const noStatusFilter = earlyLeaveOnly || lateOnly;
    const query = noStatusFilter ? "" : `?status=${encodeURIComponent(status ?? "")}`;
    apiFetch<{ employees: DayDetailEntry[] | null }>(`/api/v1/dashboard/day-detail${query}`)
      .then((d) => {
        const all = d.employees ?? [];
        if (earlyLeaveOnly) setEntries(all.filter((e) => e.early_leave_minutes > 0));
        else if (lateOnly) setEntries(all.filter((e) => e.late_minutes > 0));
        else setEntries(all);
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load this list"));
  }, [status, earlyLeaveOnly, lateOnly]);

  const lateEntries = highlightLate ? (entries ?? []).filter((e) => e.late_minutes > 0) : [];
  const otherEntries = highlightLate ? (entries ?? []).filter((e) => e.late_minutes === 0) : entries ?? [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4" onClick={onClose}>
      <div
        className="flex max-h-[80vh] w-full max-w-lg flex-col overflow-hidden rounded-xl bg-white shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-slate-200 px-5 py-3.5">
          <div>
            <h2 className="text-base font-semibold text-slate-900">{title}</h2>
            <p className="text-xs text-slate-400">
              {entries ? `${entries.length} employee${entries.length === 1 ? "" : "s"}` : "Loading…"}
            </p>
          </div>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600" aria-label="Close">
            <X size={20} />
          </button>
        </div>

        <div className="overflow-y-auto">
          {error && <p className="px-5 py-6 text-sm text-error">{error}</p>}
          {!error && entries === null && <p className="px-5 py-6 text-center text-sm text-slate-400">Loading&hellip;</p>}
          {!error && entries?.length === 0 && (
            <p className="px-5 py-6 text-center text-sm text-slate-400">No one matches this right now.</p>
          )}

          {highlightLate && lateEntries.length > 0 && (
            <div className="border-b border-warning/30 bg-warning-bg">
              <p className="flex items-center gap-1.5 px-5 pt-3 text-xs font-semibold uppercase tracking-wide text-warning">
                <WarningCircle size={14} weight="fill" />
                Arrived Late ({lateEntries.length})
              </p>
              <ul className="divide-y divide-warning/20">
                {lateEntries.map((e) => (
                  <EntryRow key={e.employee_id} e={e} />
                ))}
              </ul>
            </div>
          )}

          {otherEntries.length > 0 && (
            <>
              {highlightLate && lateEntries.length > 0 && (
                <p className="px-5 pt-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
                  On Time ({otherEntries.length})
                </p>
              )}
              <ul className="divide-y divide-slate-100">
                {otherEntries.map((e) => (
                  <EntryRow
                    key={e.employee_id}
                    e={e}
                    mode={showBothTimes ? "both" : earlyLeaveOnly ? "departure" : "arrival"}
                  />
                ))}
              </ul>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
