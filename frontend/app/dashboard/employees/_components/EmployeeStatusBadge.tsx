import type { EmployeeStatus } from "./types";

const STATUS_STYLES: Record<EmployeeStatus, string> = {
  ACTIVE: "bg-success-bg text-success",
  ON_NOTICE: "bg-warning-bg text-warning",
  INACTIVE: "bg-slate-100 text-slate-500",
  SUSPENDED: "bg-warning-bg text-warning",
  RESIGNED: "bg-slate-100 text-slate-500",
  TERMINATED: "bg-error-bg text-error",
};

const STATUS_LABELS: Record<EmployeeStatus, string> = {
  ACTIVE: "Active",
  ON_NOTICE: "On Notice",
  INACTIVE: "Inactive",
  SUSPENDED: "Suspended",
  RESIGNED: "Resigned",
  TERMINATED: "Terminated",
};

export function EmployeeStatusBadge({ status }: { status: EmployeeStatus }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold ${STATUS_STYLES[status]}`}>
      {STATUS_LABELS[status]}
    </span>
  );
}
