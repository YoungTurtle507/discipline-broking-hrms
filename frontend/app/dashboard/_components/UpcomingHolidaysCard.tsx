"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { formatDate } from "./my/types";

type Holiday = { date: string; name: string };

function daysUntil(iso: string): number {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const target = new Date(iso + "T00:00:00");
  return Math.round((target.getTime() - today.getTime()) / 86_400_000);
}

export function UpcomingHolidaysCard() {
  const [holidays, setHolidays] = useState<Holiday[] | null>(null);

  useEffect(() => {
    apiFetch<{ holidays: Holiday[] | null }>("/api/v1/dashboard/holidays?limit=3")
      .then((d) => setHolidays(d.holidays ?? []))
      .catch(() => setHolidays([]));
  }, []);

  return (
    <div className="border border-slate-200 bg-white">
      <div className="border-b border-slate-200 bg-slate-50 px-4 py-2">
        <h2 className="text-[12.5px] font-semibold uppercase tracking-wide text-slate-800">Upcoming Holidays</h2>
      </div>

      {holidays === null ? (
        <p className="px-4 py-6 text-center text-xs text-slate-400">Loading&hellip;</p>
      ) : holidays.length === 0 ? (
        <p className="px-4 py-6 text-center text-xs text-slate-400">No holidays scheduled yet.</p>
      ) : (
        <div className="flex flex-wrap">
          {holidays.map((h) => {
            const until = daysUntil(h.date);
            return (
              <div key={h.date} className="min-w-[160px] flex-1 border-r border-slate-100 px-4 py-3 last:border-r-0">
                <span className="font-mono text-[10.5px] font-semibold uppercase tracking-wide text-warning">
                  {formatDate(h.date)}
                </span>
                <p className="mt-1.5 text-[13px] font-semibold leading-snug text-slate-800">{h.name}</p>
                <p className="mt-0.5 text-[11px] text-slate-500">
                  {until === 0 ? "Today" : until === 1 ? "Tomorrow" : `In ${until} days`}
                </p>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
