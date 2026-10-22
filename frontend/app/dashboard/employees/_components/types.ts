export type EmployeeStatus =
  | "ACTIVE"
  | "INACTIVE"
  | "ON_NOTICE"
  | "RESIGNED"
  | "TERMINATED"
  | "SUSPENDED";

export const STATUS_OPTIONS: EmployeeStatus[] = [
  "ACTIVE",
  "INACTIVE",
  "ON_NOTICE",
  "RESIGNED",
  "TERMINATED",
  "SUSPENDED",
];

export type Employee = {
  id: number;
  employee_code: string;
  full_name: string;
  email: string | null;
  phone: string;
  photo_url: string;
  joining_date: string;
  department_id: number | null;
  department_name: string | null;
  designation_id: number | null;
  designation_title: string | null;
  position_id: number | null;
  position_title: string | null;
  shift_id: number | null;
  shift_name: string | null;
  reporting_manager_id: number | null;
  reporting_manager_name: string | null;
  status: EmployeeStatus;
  last_working_day: string | null;
  // The employee's own login account, if an admin has issued one. null means
  // they can't sign in yet.
  user_id: number | null;
  login_username?: string | null;
  created_at: string;
  updated_at: string;
  // Lets this specific employee mark their own attendance from their own
  // dashboard with no face verification — for someone with no camera to scan
  // against (e.g. working from home). A per-employee flag, not a role-wide one.
  remote_self_checkin: boolean;
  // True once every required document (Offer Letter, Aadhar Card, PAN Card,
  // Marksheet) has been uploaded.
  documents_complete: boolean;
  // null for any employee added before this field existed.
  date_of_birth: string | null;
  address: string;
};

// Suggested default notice period, used only to prefill the last-working-day
// date picker — admins can pick any date.
export const DEFAULT_NOTICE_PERIOD_DAYS = 60;

export type Shift = {
  id: number;
  name: string;
  start_time: string;
  end_time: string;
  grace_period_minutes: number;
  is_active: boolean;
};

export type Department = { id: number; name: string; is_active: boolean };
export type Designation = {
  id: number;
  title: string;
  department_id: number;
  department_name: string;
  is_active: boolean;
};
export type Position = { id: number; title: string; description: string; is_active: boolean };

export type AttendanceDirection = "IN" | "OUT";

// LATE_COMING is only ever set via a manual correction on an IN — never by
// the live kiosk, which decides lateness from the arrival time alone.
export type OutReason = "HALF_DAY" | "LUNCH" | "PERSONAL" | "EARLY_LEAVE" | "LATE_COMING";

export type AttendanceEvent = {
  id: number;
  employee_id: number;
  employee_name: string;
  direction: AttendanceDirection;
  event_time: string;
  confidence: number;
  model_name: string;
  model_version: string;
  verified_by: number | null;
  // Set only on an OUT scanned between 10:31 AM and 7:00 PM, where the kiosk asks
  // why the employee is going out.
  out_reason?: OutReason | null;
  note?: string;
  // Path to the stamped capture photo for this event, null for anything
  // without one (a MANUAL correction, or an event from before this existed).
  // Serve via GET /api/v1/attendance/events/:id/photo, not read directly.
  photo_path?: string | null;
};

export type EventLabel = "Office In" | "Office Out" | "Lunch In" | "Lunch Out" | "Personal In" | "Personal Out";

const BREAK_LABEL: Partial<Record<OutReason, "Lunch" | "Personal">> = {
  LUNCH: "Lunch",
  PERSONAL: "Personal",
};

// Labels events the way the office actually talks about them rather than raw
// IN/OUT: an OUT marked Lunch or Personal gets that as its label, and the
// return IN gets the same label — preferring its own reason if the employee
// (or HR) tagged it explicitly, otherwise falling back to whichever break the
// immediately preceding OUT declared, so a face-scanned return with no reason
// of its own still reads as "Lunch In"/"Personal In" rather than plain
// "Office In". Everything else is a plain Office In/Out. Callers pass in
// same-day events for one or many employees (e.g. a company-wide "today"
// feed) — grouping by employee_id happens internally.
export function labelAttendanceEvents(events: AttendanceEvent[]): Map<number, EventLabel> {
  const labels = new Map<number, EventLabel>();
  const byEmployee = new Map<number, AttendanceEvent[]>();
  for (const e of events) {
    if (!byEmployee.has(e.employee_id)) byEmployee.set(e.employee_id, []);
    byEmployee.get(e.employee_id)!.push(e);
  }
  for (const evs of byEmployee.values()) {
    const asc = [...evs].sort((a, b) => a.event_time.localeCompare(b.event_time));
    asc.forEach((e, i) => {
      if (e.direction === "OUT") {
        const kind = e.out_reason ? BREAK_LABEL[e.out_reason] : undefined;
        labels.set(e.id, kind ? `${kind} Out` : "Office Out");
      } else {
        const own = e.out_reason ? BREAK_LABEL[e.out_reason] : undefined;
        const prev = asc[i - 1];
        const inherited = prev?.direction === "OUT" && prev.out_reason ? BREAK_LABEL[prev.out_reason] : undefined;
        const kind = own ?? inherited;
        labels.set(e.id, kind ? `${kind} In` : "Office In");
      }
    });
  }
  return labels;
};

export type DailyAttendanceStatus =
  | "PRESENT"
  | "ABSENT"
  | "HALF_DAY"
  | "HOLIDAY"
  | "WEEKLY_OFF"
  | "ON_LEAVE"
  | "INACTIVE";

export type DailyAttendanceResult = {
  employee_id: number;
  date: string;
  status: DailyAttendanceStatus;
  first_in: string | null;
  last_out: string | null;
  working_minutes: number;
  late_minutes: number;
  early_leave_minutes: number;
  overtime_minutes: number;
  exceptions: string[];
  calculation_version: string;
  calculated_at: string;
  is_excused_absence: boolean;
};
