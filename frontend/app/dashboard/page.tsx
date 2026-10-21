"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { motion, useReducedMotion, type Variants } from "framer-motion";
import { StatCard } from "./_components/StatCard";
import { AttendanceChart } from "./_components/AttendanceChart";
import { TodaysAttendance } from "./_components/TodaysAttendance";
import { QuickActions } from "./_components/QuickActions";
import { RecentActivities } from "./_components/RecentActivities";
import { DayDetailModal } from "./_components/DayDetailModal";
import { UpcomingHolidaysCard } from "./_components/UpcomingHolidaysCard";
import { UpcomingBirthdaysCard } from "./_components/UpcomingBirthdaysCard";
import { DigitalClock } from "./_components/DigitalClock";
import { useSession } from "./_components/SessionProvider";
import { apiFetch } from "@/lib/api";

const MONTH_NAMES = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Mirrors calculation.DaySummary: the dashboard now reports the engine's own
// numbers rather than a parallel computation over raw events.
type DashboardSummary = {
  date: string;
  total_employees: number;
  // Everyone who turned up, full day or half day — what the Present card shows.
  at_work: number;
  present: number;
  absent: number;
  half_day: number;
  on_leave: number;
  holiday: number;
  weekly_off: number;
  late: number;
  early_leave: number;
  missing_out: number;
  overtime: number;
  working_minutes: number;
  overtime_minutes: number;
};

// This duplicate has no personal employee login at all (see auth model docs)
// — every signed-in session is either Super Admin or the shared kiosk
// account, both of which see the same company-wide dashboard.
export default function DashboardPage() {
  const { loading } = useSession();

  if (loading) {
    return <p className="text-sm text-slate-400">Loading&hellip;</p>;
  }

  return <CompanyDashboard />;
}

function greeting(hour: number) {
  if (hour < 12) return "Good morning";
  if (hour < 17) return "Good afternoon";
  return "Good evening";
}

const container: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.03 } },
};
const item: Variants = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { duration: 0.15 } },
};

function CompanyDashboard() {
  const { me } = useSession();
  const reduceMotion = useReducedMotion();
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  // Computed on mount, not at module scope: the server and the browser can
  // have different locales/timezones (or render a moment apart), so
  // formatting this during SSR risks the classic hydration text mismatch —
  // rendering it only after mount guarantees server and client agree.
  const [todayLabel, setTodayLabel] = useState<string | null>(null);
  const [greetingWord, setGreetingWord] = useState<string>("Welcome");
  const [drill, setDrill] = useState<{
    title: string;
    status?: string;
    highlightLate?: boolean;
    earlyLeaveOnly?: boolean;
    showBothTimes?: boolean;
  } | null>(null);

  useEffect(() => {
    apiFetch<DashboardSummary>("/api/v1/dashboard/summary")
      .then(setSummary)
      .catch(() => {});
    const now = new Date();
    // Spelled out from local getters rather than toLocaleDateString's format
    // options, so it reads exactly "Mon, Sep 21 2026" regardless of the
    // viewer's browser locale — this one header is deliberately not in the
    // app's usual DD/MM/YYYY, so it can't rely on the shared formatDate either.
    const weekday = now.toLocaleDateString(undefined, { weekday: "short" });
    const month = MONTH_NAMES[now.getMonth()];
    setTodayLabel(`${weekday}, ${month} ${now.getDate()}, ${now.getFullYear()}`);
    setGreetingWord(greeting(now.getHours()));
  }, []);

  const total = summary?.total_employees ?? 0;
  const pct = (n: number) => (total > 0 ? Math.round((n / total) * 1000) / 10 : 0);
  const displayName = me?.full_name;

  return (
    <motion.div
      variants={container}
      initial={reduceMotion ? false : "hidden"}
      animate="show"
      className="flex flex-col gap-4 @min-[1100px]:flex-row"
    >
      <div className="min-w-0 flex-1 @container">
        <motion.div
          variants={item}
          className="mb-4 flex flex-col justify-between gap-3 border border-slate-200 bg-white px-5 py-3.5 @min-[640px]:flex-row @min-[640px]:items-center"
        >
          <div>
            <h1 className="font-serif text-xl font-semibold text-slate-900">
              {greetingWord}
              {displayName ? `, ${displayName}` : ""}
            </h1>
            <p className="mt-0.5 text-xs text-slate-500">
              Here&apos;s what&apos;s happening in your organization today.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <DigitalClock />
            <span className="font-mono text-xs text-slate-500">{todayLabel ?? " "}</span>
            <Link
              href="/dashboard/employees"
              className="border border-brand-blue bg-brand-blue px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-brand-blue-dark"
            >
              Employees
            </Link>
          </div>
        </motion.div>

        <motion.div variants={item} className="flex flex-wrap border border-slate-200 bg-white">
          <StatCard
            label="Present"
            tone="success"
            value={summary?.at_work ?? 0}
            percentOfTotal={pct(summary?.at_work ?? 0)}
            sub={summary?.late ? `${summary.late} late arrival${summary.late === 1 ? "" : "s"}` : undefined}
            onClick={() => setDrill({ title: "Present Today", status: "PRESENT,HALF_DAY", highlightLate: true })}
          />
          <StatCard
            label="Absent"
            tone="error"
            value={summary?.absent ?? 0}
            percentOfTotal={pct(summary?.absent ?? 0)}
            sub={summary?.missing_out ? `${summary.missing_out} missing checkout` : undefined}
            onClick={() => setDrill({ title: "Absent Today", status: "ABSENT" })}
          />
          <StatCard
            label="On Leave"
            tone="accent"
            value={summary?.on_leave ?? 0}
            percentOfTotal={pct(summary?.on_leave ?? 0)}
            onClick={() => setDrill({ title: "On Leave Today", status: "ON_LEAVE" })}
          />
          <StatCard
            label="Half Day"
            tone="warning"
            value={summary?.half_day ?? 0}
            percentOfTotal={pct(summary?.half_day ?? 0)}
            sub={summary?.early_leave ? `${summary.early_leave} early leave` : undefined}
            onClick={() => setDrill({ title: "Half Day Today", status: "HALF_DAY", showBothTimes: true })}
          />
          <StatCard
            label="Early Leave"
            tone="brand"
            value={summary?.early_leave ?? 0}
            percentOfTotal={pct(summary?.early_leave ?? 0)}
            onClick={() => setDrill({ title: "Left Early Today", earlyLeaveOnly: true })}
          />
        </motion.div>

        {drill && (
          <DayDetailModal
            title={drill.title}
            status={drill.status}
            highlightLate={drill.highlightLate}
            earlyLeaveOnly={drill.earlyLeaveOnly}
            showBothTimes={drill.showBothTimes}
            onClose={() => setDrill(null)}
          />
        )}

        <motion.div variants={item} className="mt-4">
          <QuickActions />
        </motion.div>

        <motion.div variants={item} className="mt-4 grid grid-cols-1 gap-4 @min-[640px]:grid-cols-3">
          <div className="@min-[640px]:col-span-2">
            <AttendanceChart />
          </div>
          <TodaysAttendance />
        </motion.div>

        <motion.div variants={item} className="mt-4">
          <UpcomingHolidaysCard />
        </motion.div>

        <motion.div variants={item} className="mt-4">
          <UpcomingBirthdaysCard />
        </motion.div>
      </div>

      <motion.div variants={item} className="flex w-full flex-col gap-4 @min-[1100px]:w-80 @min-[1100px]:shrink-0">
        <RecentActivities />
      </motion.div>
    </motion.div>
  );
}
