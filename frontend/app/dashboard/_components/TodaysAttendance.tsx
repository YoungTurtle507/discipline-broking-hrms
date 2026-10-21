"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "@phosphor-icons/react/ssr";
import { Avatar } from "@/components/Avatar";
import { apiFetch } from "@/lib/api";
import { labelAttendanceEvents, type AttendanceEvent } from "../employees/_components/types";

export function TodaysAttendance() {
  const [events, setEvents] = useState<AttendanceEvent[]>([]);
  const labels = labelAttendanceEvents(events);

  useEffect(() => {
    apiFetch<{ events: AttendanceEvent[] | null }>("/api/v1/attendance/today")
      .then((d) => setEvents(d.events ?? []))
      .catch(() => {});
  }, []);

  return (
    <div className="@container border border-slate-200 bg-white">
      <div className="flex items-center justify-between gap-3 border-b border-slate-200 bg-slate-50 px-4 py-2">
        <h2 className="text-nowrap text-[12.5px] font-semibold uppercase tracking-wide text-slate-800">
          Today&apos;s Attendance
        </h2>
        <Link
          href="/dashboard/attendance"
          className="flex shrink-0 items-center gap-1 text-nowrap text-xs font-medium text-brand-blue hover:underline"
        >
          View All
          <ArrowRight size={12} />
        </Link>
      </div>

      {events.length === 0 ? (
        <p className="px-4 py-6 text-center text-xs text-slate-400">No attendance recorded yet today.</p>
      ) : (
        <ul>
          {events.slice(0, 6).map((ev) => (
            <li key={ev.id} className="flex items-center gap-2 border-b border-slate-100 px-4 py-2.5 last:border-b-0">
              <Avatar name={ev.employee_name} employeeId={ev.employee_id} size={30} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-[13px] font-semibold text-slate-800">{ev.employee_name}</p>
                <p className="truncate font-mono text-[11px] text-slate-400">
                  {new Date(ev.event_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                </p>
              </div>
              <span
                className={`shrink-0 border px-2 py-0.5 font-mono text-[10.5px] font-semibold ${
                  ev.direction === "IN"
                    ? "border-success/30 bg-success-bg text-success"
                    : "border-warning/30 bg-warning-bg text-warning"
                }`}
              >
                {labels.get(ev.id) ?? ev.direction}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
