"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "@phosphor-icons/react/ssr";
import { apiFetch } from "@/lib/api";
import { labelAttendanceEvents, type AttendanceEvent, type Employee } from "../employees/_components/types";
import { formatDate } from "./my/types";

type ActivityKind = "attendance" | "employee";

type Activity = {
  id: string;
  kind: ActivityKind;
  message: string;
  timestamp: Date;
};

const ACTIVITY_STYLES: Record<ActivityKind, { dot: string }> = {
  attendance: { dot: "bg-success" },
  employee: { dot: "bg-brand-blue" },
};

function formatTimestamp(d: Date): string {
  const isToday = d.toDateString() === new Date().toDateString();
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (isToday) return `Today, ${time}`;
  // Local calendar fields, never toISOString — that converts to UTC first,
  // which can silently shift the date by a day in any timezone ahead of UTC.
  const localIso = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  return `${formatDate(localIso)}, ${time}`;
}

export function RecentActivities() {
  const [activities, setActivities] = useState<Activity[] | null>(null);

  useEffect(() => {
    Promise.all([
      apiFetch<{ events: AttendanceEvent[] | null }>("/api/v1/attendance/today"),
      apiFetch<{ employees: Employee[] | null }>("/api/v1/employees"),
    ])
      .then(([attendanceData, employeeData]) => {
        const events = attendanceData.events ?? [];
        const labels = labelAttendanceEvents(events);
        const attendanceActivities: Activity[] = events.map((ev) => ({
          id: `att-${ev.id}`,
          kind: "attendance",
          message: `${ev.employee_name} marked attendance (${labels.get(ev.id) ?? ev.direction})`,
          timestamp: new Date(ev.event_time),
        }));

        const recentEmployees = [...(employeeData.employees ?? [])]
          .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
          .slice(0, 5)
          .map(
            (emp): Activity => ({
              id: `emp-${emp.id}`,
              kind: "employee",
              message: `New employee added: ${emp.full_name}`,
              timestamp: new Date(emp.created_at),
            })
          );

        const merged = [...attendanceActivities, ...recentEmployees]
          .sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime())
          .slice(0, 6);
        setActivities(merged);
      })
      .catch(() => setActivities([]));
  }, []);

  return (
    <div className="border border-slate-200 bg-white">
      <div className="flex items-center justify-between gap-3 border-b border-slate-200 bg-slate-50 px-4 py-2">
        <h2 className="text-nowrap text-[12.5px] font-semibold uppercase tracking-wide text-slate-800">
          Recent Activities
        </h2>
        <Link
          href="/dashboard/attendance"
          className="flex shrink-0 items-center gap-1 text-nowrap text-xs font-medium text-brand-blue hover:underline"
        >
          View All
          <ArrowRight size={12} />
        </Link>
      </div>

      {!activities || activities.length === 0 ? (
        <p className="px-4 py-6 text-center text-xs text-slate-400">Nothing to show yet.</p>
      ) : (
        <ul>
          {activities.map((activity) => {
            const { dot } = ACTIVITY_STYLES[activity.kind];
            return (
              <li key={activity.id} className="flex items-start gap-2.5 border-b border-slate-100 px-4 py-2.5 last:border-b-0">
                <span className={`mt-1.5 h-1.5 w-1.5 shrink-0 ${dot}`} />
                <div className="min-w-0">
                  <p className="text-[12.5px] text-slate-700">{activity.message}</p>
                  <p className="mt-0.5 font-mono text-[10.5px] text-slate-400">{formatTimestamp(activity.timestamp)}</p>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
