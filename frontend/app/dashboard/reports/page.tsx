"use client";

import { useEffect, useState } from "react";
import { ClockCounterClockwise, Image as ImageIcon, SignIn, SignOut, X } from "@phosphor-icons/react";
import { apiFetch, getBackendUrl, ApiError } from "@/lib/api";
import { Avatar } from "@/components/Avatar";
import { EmployeeSearchPicker } from "../_components/EmployeeSearchPicker";
import { formatDate } from "../_components/my/types";
import type { AttendanceEvent, Employee, OutReason } from "../employees/_components/types";

const OUT_REASON_LABEL: Record<OutReason, string> = {
  HALF_DAY: "Half Day",
  LUNCH: "Lunch",
  PERSONAL: "Personal",
  EARLY_LEAVE: "Early Leave",
  LATE_COMING: "Late Coming",
};

type Tab = "attendance" | "photos";

function isoDate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

export default function ReportsPage() {
  const [tab, setTab] = useState<Tab>("attendance");
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [employeeFilter, setEmployeeFilter] = useState<Employee | null>(null);
  const [from, setFrom] = useState(() => {
    const d = new Date();
    d.setDate(d.getDate() - 30);
    return isoDate(d);
  });
  const [to, setTo] = useState(() => isoDate(new Date()));
  const [events, setEvents] = useState<AttendanceEvent[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState<AttendanceEvent | null>(null);

  useEffect(() => {
    apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
      .then((d) => setEmployees(d.employees ?? []))
      .catch(() => {});
  }, []);

  async function load() {
    setError(null);
    try {
      const params = new URLSearchParams({ from, to });
      if (employeeFilter) params.set("employee_id", String(employeeFilter.id));
      const data = await apiFetch<{ events: AttendanceEvent[] | null }>(`/api/v1/attendance/report?${params}`);
      setEvents(data.events ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load report");
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [from, to, employeeFilter]);

  const photoEvents = (events ?? []).filter((e) => e.photo_path);

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Reports</h1>
        <p className="mt-1 text-sm text-slate-500">Attendance history and captured photos across the team.</p>
      </div>

      <div className="mb-5 flex flex-wrap items-end gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-card">
        <div className="min-w-[220px] flex-1">
          <label className="mb-1 block text-xs font-semibold text-slate-600">Employee</label>
          <EmployeeSearchPicker
            employees={employees}
            selected={employeeFilter}
            onSelect={setEmployeeFilter}
            placeholder="All employees…"
            changeLabel="Clear"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-semibold text-slate-600">From</label>
          <input type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} className="input" />
        </div>
        <div>
          <label className="mb-1 block text-xs font-semibold text-slate-600">To</label>
          <input type="date" value={to} min={from} max={isoDate(new Date())} onChange={(e) => setTo(e.target.value)} className="input" />
        </div>
      </div>

      <div className="mb-5 flex gap-1 border-b border-slate-200">
        <button
          type="button"
          onClick={() => setTab("attendance")}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-semibold transition-colors ${
            tab === "attendance" ? "border-brand-blue text-brand-blue" : "border-transparent text-slate-500 hover:text-slate-800"
          }`}
        >
          <ClockCounterClockwise size={16} />
          Attendance
        </button>
        <button
          type="button"
          onClick={() => setTab("photos")}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-semibold transition-colors ${
            tab === "photos" ? "border-brand-blue text-brand-blue" : "border-transparent text-slate-500 hover:text-slate-800"
          }`}
        >
          <ImageIcon size={16} />
          Captured Photos
        </button>
      </div>

      {error && <p className="mb-4 text-sm text-error">{error}</p>}

      {events === null ? (
        <p className="py-10 text-center text-sm text-slate-400">Loading…</p>
      ) : tab === "attendance" ? (
        <div className="rounded-xl border border-slate-200 bg-white shadow-card">
          {events.length === 0 ? (
            <p className="py-10 text-center text-sm text-slate-400">No attendance in this range.</p>
          ) : (
            <div className="divide-y divide-slate-100">
              {events.map((ev) => (
                <div key={ev.id} className="flex items-center gap-3 px-5 py-3">
                  <Avatar name={ev.employee_name} employeeId={ev.employee_id} size={32} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-slate-800">{ev.employee_name}</p>
                    <p className="truncate text-xs text-slate-400">
                      {formatDate(ev.event_time)} &middot;{" "}
                      {new Date(ev.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                      {ev.out_reason && ` (${OUT_REASON_LABEL[ev.out_reason]})`}
                      {ev.note ? ` — ${ev.note}` : ""}
                    </p>
                  </div>
                  <span
                    className={`flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ${
                      ev.direction === "IN" ? "bg-success-bg text-success" : "bg-warning-bg text-warning"
                    }`}
                  >
                    {ev.direction === "IN" ? <SignIn size={12} weight="bold" /> : <SignOut size={12} weight="bold" />}
                    {ev.direction}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : photoEvents.length === 0 ? (
        <p className="py-10 text-center text-sm text-slate-400">No captured photos in this range.</p>
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {photoEvents.map((ev) => (
            <button
              key={ev.id}
              type="button"
              onClick={() => setPreview(ev)}
              className="group overflow-hidden rounded-xl border border-slate-200 bg-white text-left shadow-card transition-shadow hover:shadow-md"
            >
              <div className="aspect-square w-full overflow-hidden bg-slate-100">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={`${getBackendUrl()}/api/v1/attendance/events/${ev.id}/photo`}
                  alt={`${ev.employee_name} — ${ev.direction}`}
                  className="h-full w-full object-cover transition-transform group-hover:scale-105"
                />
              </div>
              <div className="p-2.5">
                <p className="truncate text-xs font-semibold text-slate-800">{ev.employee_name}</p>
                <p className="truncate text-[11px] text-slate-400">
                  {ev.direction} &middot; {formatDate(ev.event_time)}
                </p>
              </div>
            </button>
          ))}
        </div>
      )}

      {preview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/70 p-4" onClick={() => setPreview(null)}>
          <div className="max-w-lg rounded-xl bg-white p-4 shadow-xl" onClick={(e) => e.stopPropagation()}>
            <div className="mb-3 flex items-center justify-between">
              <div>
                <p className="text-sm font-semibold text-slate-800">{preview.employee_name}</p>
                <p className="text-xs text-slate-400">
                  {preview.direction} &middot; {formatDate(preview.event_time)} &middot;{" "}
                  {new Date(preview.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                </p>
              </div>
              <button type="button" onClick={() => setPreview(null)} className="text-slate-400 hover:text-slate-600">
                <X size={20} />
              </button>
            </div>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={`${getBackendUrl()}/api/v1/attendance/events/${preview.id}/photo`}
              alt={`${preview.employee_name} — ${preview.direction}`}
              className="max-h-[70vh] w-full rounded-lg object-contain"
            />
          </div>
        </div>
      )}
    </div>
  );
}
