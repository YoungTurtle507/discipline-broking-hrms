export type DayStatus =
  | "PRESENT"
  | "ABSENT"
  | "HALF_DAY"
  | "HOLIDAY"
  | "WEEKLY_OFF"
  | "ON_LEAVE"
  | "INACTIVE";

export type MyDay = {
  employee_id: number;
  date: string;
  status: DayStatus;
  first_in: string | null;
  last_out: string | null;
  working_minutes: number;
  late_minutes: number;
  early_leave_minutes: number;
  overtime_minutes: number;
  exceptions: string[] | null;
  calculation_version: string;
  compoff_earned: boolean;
};

export type MyPunch = { id: number; direction: "IN" | "OUT"; event_time: string; out_reason: string | null };

export type MyToday = {
  date: string;
  punches: MyPunch[];
  day?: MyDay;
  next_direction?: "IN" | "OUT";
};

export type PunchLabel = "Office In" | "Office Out" | "Lunch In" | "Lunch Out" | "Personal In" | "Personal Out";

const BREAK_KIND: Partial<Record<string, "Lunch" | "Personal">> = { LUNCH: "Lunch", PERSONAL: "Personal" };

// Labels an employee's own punches the same way labelAttendanceEvents (in
// employees/_components/types.ts) labels everyone else's — an OUT tagged
// Lunch/Personal gets that as its label, and the return IN gets the same
// label, preferring its own reason if tagged explicitly, otherwise inheriting
// from the immediately preceding OUT. Punches are assumed already sorted by
// event_time ascending (PunchesOn's own SQL order), and — unlike the shared
// admin version — never need to group by employee, since this is always one
// employee's own list.
export function labelPunches(punches: MyPunch[]): Map<number, PunchLabel> {
  const labels = new Map<number, PunchLabel>();
  punches.forEach((p, i) => {
    if (p.direction === "OUT") {
      const kind = p.out_reason ? BREAK_KIND[p.out_reason] : undefined;
      labels.set(p.id, kind ? `${kind} Out` : "Office Out");
    } else {
      const own = p.out_reason ? BREAK_KIND[p.out_reason] : undefined;
      const prev = punches[i - 1];
      const inherited = prev?.direction === "OUT" && prev.out_reason ? BREAK_KIND[prev.out_reason] : undefined;
      const kind = own ?? inherited;
      labels.set(p.id, kind ? `${kind} In` : "Office In");
    }
  });
  return labels;
}

export type MySummary = {
  present: number;
  absent: number;
  half_day: number;
  on_leave: number;
  holiday: number;
  weekly_off: number;
  late_arrivals: number;
  early_departures: number;
  needs_correction: number;
  working_minutes: number;
  overtime_minutes: number;
  compoff_earned: number;
};

export type MyAttendance = { from: string; to: string; days: MyDay[]; summary: MySummary };

export const STATUS_LABEL: Record<DayStatus, string> = {
  PRESENT: "Present",
  ABSENT: "Absent",
  HALF_DAY: "Half Day",
  HOLIDAY: "Holiday",
  WEEKLY_OFF: "Weekly Off",
  ON_LEAVE: "On Leave",
  INACTIVE: "Not Employed",
};

export const STATUS_STYLE: Record<DayStatus, string> = {
  PRESENT: "bg-success-bg text-success",
  ABSENT: "bg-error-bg text-error",
  HALF_DAY: "bg-warning-bg text-warning",
  HOLIDAY: "bg-amber-bg text-amber",
  WEEKLY_OFF: "bg-slate-100 text-slate-500",
  ON_LEAVE: "bg-accent-purple-bg text-accent-purple",
  INACTIVE: "bg-slate-100 text-slate-400",
};

// Exception codes come from the engine; these are the ones an employee can act
// on or should understand. Anything unmapped falls back to the raw code.
export const EXCEPTION_LABEL: Record<string, string> = {
  MISSING_OUT: "No check-out recorded",
  INVALID_SEQUENCE: "Scans out of order",
  DUPLICATE_IN: "Duplicate check-in",
  DUPLICATE_OUT: "Duplicate check-out",
  LOW_CONFIDENCE: "Low face-match confidence",
  LATE_STREAK_COUNTED: "Counted toward monthly late limit",
  LATE_STREAK_HALF_DAY: "Third late arrival this month — half day",
  LATE_ARRIVAL_HALF_DAY: "Arrived past the half-day cutoff",
  DECLARED_HALF_DAY: "Marked as a half day",
  LATE_PASS_USED: "Excused by staying late the previous day",
  UNAPPROVED_ABSENCE: "Absent with no approved leave",
};

export function formatMinutes(total: number): string {
  if (total <= 0) return "0h 00m";
  const h = Math.floor(total / 60);
  const m = total % 60;
  return `${h}h ${String(m).padStart(2, "0")}m`;
}

export function formatClock(iso: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

// Renders any ISO date or date-time string as DD/MM/YYYY for display. Never
// use this on values headed back into an API call or a native date input —
// those stay YYYY-MM-DD.
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const datePart = iso.slice(0, 10);
  const [year, month, day] = datePart.split("-");
  if (!year || !month || !day) return "—";
  return `${day}/${month}/${year}`;
}
