// Parses a handful of common attendance questions typed straight into the
// top search bar ("how many days did chetan come late this month") into an
// employee + a metric + a date range, so the Topbar can answer directly
// instead of sending HR through Reports for something this specific. This is
// deliberately keyword-based, not real NLP — a fixed, growable list of
// phrases mapped to the exact fields reports.AttendanceSummary already
// computes (see backend/internal/reports/handlers.go), so an answer here can
// never disagree with what the Reports page would show for the same range.
import type { Employee } from "../employees/_components/types";

export type IntentKey =
  | "half_day"
  | "early_leave"
  | "missing_out"
  | "overtime"
  | "working_days"
  | "absent"
  | "leave"
  | "present"
  | "late";

type Intent = {
  key: IntentKey;
  keywords: string[];
  field: string; // key into EmployeeAttendanceRow (see reports handlers.go)
  describe: (value: number) => string;
};

// Order matters — checked top to bottom, first match wins. Multi-word/more
// specific phrases come first so e.g. "early leave" doesn't get swallowed by
// the generic "leave" intent below it.
const INTENTS: Intent[] = [
  {
    key: "half_day",
    keywords: ["half day", "half-day", "halfday", "half days"],
    field: "half_days",
    describe: (n) => `${n} half day${n === 1 ? "" : "s"}`,
  },
  {
    key: "early_leave",
    keywords: ["early leave", "left early", "leave early", "early departure", "early out", "leaving early"],
    field: "early_leave_count",
    describe: (n) => `left early ${n} time${n === 1 ? "" : "s"}`,
  },
  {
    key: "missing_out",
    keywords: [
      "missing out", "missed out", "forgot to scan", "forgot to mark", "forgot to punch",
      "missed punch", "no checkout", "didn't scan out", "didnt scan out", "office out missing",
    ],
    field: "missing_out_count",
    describe: (n) => `forgot to mark office out ${n} time${n === 1 ? "" : "s"}`,
  },
  {
    key: "overtime",
    keywords: ["overtime", "over time", "stayed late", "worked late"],
    field: "overtime_days",
    describe: (n) => `worked overtime on ${n} day${n === 1 ? "" : "s"}`,
  },
  {
    key: "working_days",
    keywords: ["working days", "total working days", "work days"],
    field: "total_working_days",
    describe: (n) => `${n} total working day${n === 1 ? "" : "s"}`,
  },
  {
    key: "absent",
    keywords: ["absent", "absence", "unapproved absence"],
    field: "uninformed_leave_days",
    describe: (n) => `was absent (no leave applied) ${n} day${n === 1 ? "" : "s"}`,
  },
  {
    key: "leave",
    keywords: ["leave", "leaves", "on leave"],
    field: "leave_days_total",
    describe: (n) => `took ${n} day${n === 1 ? "" : "s"} of leave`,
  },
  {
    key: "present",
    keywords: ["present", "came to office", "came to work", "attendance"],
    field: "present_days",
    describe: (n) => `was present for ${n} day${n === 1 ? "" : "s"}`,
  },
  {
    key: "late",
    keywords: ["late", "come late", "came late", "late comer", "late arrival", "late coming", "coming late"],
    field: "late_count",
    describe: (n) => `was late ${n} time${n === 1 ? "" : "s"}`,
  },
];

// Builds YYYY-MM-DD from the Date's own local year/month/day — never
// toISOString(), which converts to UTC first and silently rolls the date
// back a day for any timezone ahead of UTC (e.g. IST).
function toISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function startOfWeek(d: Date): Date {
  const out = new Date(d);
  const day = out.getDay(); // 0 = Sunday
  const diff = day === 0 ? 6 : day - 1; // days since Monday
  out.setDate(out.getDate() - diff);
  return out;
}

const MONTH_NAMES = [
  "january", "february", "march", "april", "may", "june",
  "july", "august", "september", "october", "november", "december",
];

// Resolves a free-text date phrase to a concrete [from, to] range (both
// YYYY-MM-DD, inclusive). Defaults to "this month to date" when nothing in
// the query names a range at all — the same default a fresh Reports visit
// would reasonably use.
function parseDateRange(text: string, today: Date): { from: string; to: string; rangeLabel: string } {
  const todayISO = toISODate(today);

  const lastNDays = text.match(/last (\d+) days?/);
  if (lastNDays) {
    const n = parseInt(lastNDays[1], 10);
    const from = new Date(today);
    from.setDate(from.getDate() - (n - 1));
    return { from: toISODate(from), to: todayISO, rangeLabel: `the last ${n} days` };
  }

  if (text.includes("yesterday")) {
    const y = new Date(today);
    y.setDate(y.getDate() - 1);
    return { from: toISODate(y), to: toISODate(y), rangeLabel: "yesterday" };
  }
  if (text.includes("today")) {
    return { from: todayISO, to: todayISO, rangeLabel: "today" };
  }
  if (text.includes("last week")) {
    const thisMonday = startOfWeek(today);
    const lastMonday = new Date(thisMonday);
    lastMonday.setDate(lastMonday.getDate() - 7);
    const lastSunday = new Date(lastMonday);
    lastSunday.setDate(lastSunday.getDate() + 6);
    return { from: toISODate(lastMonday), to: toISODate(lastSunday), rangeLabel: "last week" };
  }
  if (text.includes("this week")) {
    return { from: toISODate(startOfWeek(today)), to: todayISO, rangeLabel: "this week" };
  }
  if (text.includes("last month")) {
    const firstOfThisMonth = new Date(today.getFullYear(), today.getMonth(), 1);
    const lastMonthEnd = new Date(firstOfThisMonth.getTime() - 1);
    const lastMonthStart = new Date(lastMonthEnd.getFullYear(), lastMonthEnd.getMonth(), 1);
    return { from: toISODate(lastMonthStart), to: toISODate(lastMonthEnd), rangeLabel: "last month" };
  }
  if (text.includes("last year")) {
    const y = today.getFullYear() - 1;
    return { from: `${y}-01-01`, to: `${y}-12-31`, rangeLabel: "last year" };
  }
  if (text.includes("this year")) {
    return { from: `${today.getFullYear()}-01-01`, to: todayISO, rangeLabel: "this year" };
  }

  const monthMatch = text.match(new RegExp(`\\b(${MONTH_NAMES.join("|")})\\b(?:\\s+(\\d{4}))?`));
  if (monthMatch) {
    const monthIndex = MONTH_NAMES.indexOf(monthMatch[1]);
    let year = monthMatch[2] ? parseInt(monthMatch[2], 10) : today.getFullYear();
    if (!monthMatch[2] && new Date(year, monthIndex, 1) > today) {
      year -= 1; // an unqualified month that hasn't happened yet this year means last year's
    }
    const start = new Date(year, monthIndex, 1);
    const end = new Date(year, monthIndex + 1, 0);
    const cappedEnd = end > today ? today : end;
    return {
      from: toISODate(start),
      to: toISODate(cappedEnd),
      rangeLabel: `${monthMatch[1][0].toUpperCase()}${monthMatch[1].slice(1)} ${year}`,
    };
  }

  const yearOnly = text.match(/\b(20\d{2})\b/);
  if (yearOnly) {
    const year = parseInt(yearOnly[1], 10);
    const end = year === today.getFullYear() ? todayISO : `${year}-12-31`;
    return { from: `${year}-01-01`, to: end, rangeLabel: `${year}` };
  }

  if (text.includes("this month")) {
    return { from: toISODate(new Date(today.getFullYear(), today.getMonth(), 1)), to: todayISO, rangeLabel: "this month" };
  }

  // Default: current month to date.
  return { from: toISODate(new Date(today.getFullYear(), today.getMonth(), 1)), to: todayISO, rangeLabel: "this month" };
}

// Two-tier rank: a name/code that starts with the needle outranks one that
// merely contains it — same logic as the attendance-correction modal's
// employee picker (frontend/app/dashboard/attendance/page.tsx).
function findMentionedEmployee(text: string, employees: Employee[]): Employee | null {
  const candidates = employees
    .map((e) => {
      const full = e.full_name.toLowerCase();
      const first = full.split(/\s+/)[0];
      let rank = -1;
      if (text.includes(full)) rank = 0;
      else if (new RegExp(`\\b${escapeRegExp(first)}\\b`).test(text)) rank = 1;
      return { e, rank };
    })
    .filter((r) => r.rank >= 0)
    .sort((a, b) => a.rank - b.rank);

  if (candidates.length === 0) return null;
  // Ambiguous first-name-only match (e.g. two "Yash"s) — don't guess which
  // one; let the plain name-filter results below handle it instead.
  const topRank = candidates[0].rank;
  const tied = candidates.filter((c) => c.rank === topRank);
  if (tied.length > 1) return null;
  return tied[0].e;
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export type SmartQuery = {
  employee: Employee;
  intentKey: IntentKey;
  field: string;
  describe: (value: number) => string;
  from: string;
  to: string;
  rangeLabel: string;
};

// Returns null when the query doesn't look like an attendance question at
// all (no known intent phrase found), or when it does but no single employee
// could be confidently resolved — in both cases the caller should just fall
// back to the ordinary name-filter list.
export function parseSmartQuery(query: string, employees: Employee[]): SmartQuery | null {
  const text = ` ${query.trim().toLowerCase()} `;
  if (text.trim().length < 3) return null;

  const intent = INTENTS.find((i) => i.keywords.some((k) => text.includes(k)));
  if (!intent) return null;

  const employee = findMentionedEmployee(text, employees);
  if (!employee) return null;

  const { from, to, rangeLabel } = parseDateRange(text, new Date());

  return { employee, intentKey: intent.key, field: intent.field, describe: intent.describe, from, to, rangeLabel };
}
