"use client";

import { useEffect, useState } from "react";
import { Cake } from "@phosphor-icons/react";
import { apiFetch } from "@/lib/api";
import { formatDate } from "./my/types";

type Birthday = {
  employee_id: number;
  full_name: string;
  department_name?: string;
  date_of_birth: string;
  days_until: number;
};

export function UpcomingBirthdaysCard() {
  const [birthdays, setBirthdays] = useState<Birthday[] | null>(null);

  useEffect(() => {
    apiFetch<{ birthdays: Birthday[] | null }>("/api/v1/dashboard/birthdays")
      .then((d) => setBirthdays(d.birthdays ?? []))
      .catch(() => setBirthdays([]));
  }, []);

  if (birthdays !== null && birthdays.length === 0) return null;

  return (
    <div className="border border-slate-200 bg-white">
      <div className="border-b border-slate-200 bg-slate-50 px-4 py-2">
        <h2 className="text-[12.5px] font-semibold uppercase tracking-wide text-slate-800">Upcoming Birthdays</h2>
      </div>

      {birthdays === null ? (
        <p className="px-4 py-6 text-center text-xs text-slate-400">Loading&hellip;</p>
      ) : (
        <ul className="divide-y divide-slate-100">
          {birthdays.map((b) => (
            <li key={b.employee_id} className="flex items-center gap-3 px-4 py-2.5">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent-purple/10 text-accent-purple">
                <Cake size={16} weight="fill" />
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[13px] font-semibold text-slate-800">{b.full_name}</p>
                <p className="truncate text-[11px] text-slate-400">
                  {b.department_name ?? "—"} &middot; {formatDate(b.date_of_birth)}
                </p>
              </div>
              <span className="shrink-0 rounded-full bg-light-blue px-2.5 py-1 text-[11px] font-semibold text-brand-blue">
                {b.days_until === 0 ? "Today" : b.days_until === 1 ? "Tomorrow" : `In ${b.days_until} days`}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
