"use client";

import { useEffect, useState } from "react";
import {
  ResponsiveContainer,
  ComposedChart,
  Area,
  Line,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
} from "recharts";
import { apiFetch } from "@/lib/api";

type TrendPoint = { date: string; present: number; late_coming: number; half_day: number };

const PRESENT_COLOR = "#2d5d8b";
const LATE_COMING_COLOR = "#d97706";
const HALF_DAY_COLOR = "#7c3aed";

// DD/MM chart-axis label — plain string slicing rather than Date/toLocale,
// so it can never drift a day from a UTC conversion, and keeps the same
// day-before-month order as every date elsewhere in the app. The year is
// left off only here: this is always a rolling last-7-days axis, not a
// record that needs to stand alone.
function formatDay(iso: string): string {
  const [, month, day] = iso.split("-");
  return `${day}/${month}`;
}

export function AttendanceChart() {
  const [trend, setTrend] = useState<TrendPoint[]>([]);

  useEffect(() => {
    apiFetch<{ trend: TrendPoint[] | null }>("/api/v1/dashboard/trend")
      .then((d) =>
        setTrend(
          (d.trend ?? []).map((p) => ({
            date: formatDay(p.date),
            present: p.present,
            late_coming: p.late_coming,
            half_day: p.half_day,
          }))
        )
      )
      .catch(() => {});
  }, []);

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-nowrap text-base font-semibold text-slate-900">Attendance Overview</h2>
        <div className="flex flex-wrap items-center gap-4">
          <span className="flex items-center gap-1.5 text-nowrap text-xs font-medium text-slate-500">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: PRESENT_COLOR }} />
            Present (last 7 days)
          </span>
          <span className="flex items-center gap-1.5 text-nowrap text-xs font-medium text-slate-500">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: LATE_COMING_COLOR }} />
            Late Coming
          </span>
          <span className="flex items-center gap-1.5 text-nowrap text-xs font-medium text-slate-500">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: HALF_DAY_COLOR }} />
            Half Day
          </span>
        </div>
      </div>

      <div className="h-72 w-full">
        {trend.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm text-slate-400">
            No attendance recorded in the last 7 days yet.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={trend} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="presentFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={PRESENT_COLOR} stopOpacity={0.2} />
                  <stop offset="100%" stopColor={PRESENT_COLOR} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid stroke="#dcdcd8" vertical={false} />
              <XAxis dataKey="date" axisLine={false} tickLine={false} tick={{ fill: "#9a9a96", fontSize: 12 }} dy={8} />
              <YAxis
                allowDecimals={false}
                axisLine={false}
                tickLine={false}
                tick={{ fill: "#9a9a96", fontSize: 12 }}
                width={36}
                tickMargin={4}
              />
              <Tooltip
                contentStyle={{
                  borderRadius: 12,
                  border: "1px solid #dcdcd8",
                  background: "#ffffff",
                  boxShadow: "0 4px 20px rgba(46, 50, 48, 0.1)",
                  fontSize: 13,
                }}
                labelStyle={{ color: "#16171a", fontWeight: 600, marginBottom: 4 }}
              />
              <Area
                type="monotone"
                dataKey="present"
                name="Present"
                stroke={PRESENT_COLOR}
                strokeWidth={2.5}
                fill="url(#presentFill)"
                dot={{ r: 3.5, fill: PRESENT_COLOR, strokeWidth: 0 }}
                activeDot={{ r: 5 }}
              />
              <Line
                type="monotone"
                dataKey="late_coming"
                name="Late Coming"
                stroke={LATE_COMING_COLOR}
                strokeWidth={2}
                dot={{ r: 3, fill: LATE_COMING_COLOR, strokeWidth: 0 }}
                activeDot={{ r: 4.5 }}
              />
              <Line
                type="monotone"
                dataKey="half_day"
                name="Half Day"
                stroke={HALF_DAY_COLOR}
                strokeWidth={2}
                dot={{ r: 3, fill: HALF_DAY_COLOR, strokeWidth: 0 }}
                activeDot={{ r: 4.5 }}
              />
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
}
