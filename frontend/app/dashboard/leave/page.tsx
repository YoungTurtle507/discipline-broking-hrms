"use client";

import { useEffect, useState } from "react";
import { CalendarCheck, CheckCircle, XCircle, X, Paperclip } from "@phosphor-icons/react";
import { apiFetch, apiUpload, getBackendUrl, ApiError } from "@/lib/api";
import { Avatar } from "@/components/Avatar";
import { EmployeeSearchPicker } from "../_components/EmployeeSearchPicker";
import { DateField } from "../_components/DateField";
import { formatDate } from "../_components/my/types";
import { useSession } from "../_components/SessionProvider";
import type { Employee, LeaveRequest, LeaveType } from "../employees/_components/types";

const LEAVE_TYPE_LABEL: Record<LeaveType, string> = {
  PAID: "Paid Leave",
  COMPOFF: "Comp-Off",
  UNPAID: "Unpaid Leave",
};

const STATUS_STYLES: Record<LeaveRequest["status"], string> = {
  PENDING: "bg-warning-bg text-warning",
  APPROVED: "bg-success-bg text-success",
  REJECTED: "bg-error-bg text-error",
};

export default function LeavePage() {
  const { can } = useSession();
  const canApply = can("leave.request_own");
  const canApprove = can("leave.approve");

  const [requests, setRequests] = useState<LeaveRequest[] | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [applyOpen, setApplyOpen] = useState(false);
  const [actingOn, setActingOn] = useState<number | null>(null);

  async function load() {
    try {
      const data = await apiFetch<{ requests: LeaveRequest[] | null }>("/api/v1/leave");
      setRequests(data.requests ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load leave requests");
    }
  }

  useEffect(() => {
    load();
    if (canApply) {
      apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
        .then((d) => setEmployees(d.employees ?? []))
        .catch(() => {});
    }
  }, [canApply]);

  async function decide(id: number, action: "approve" | "reject") {
    setActingOn(id);
    setError(null);
    try {
      await apiFetch(`/api/v1/leave/${id}/${action}`, { method: "POST" });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : `Failed to ${action} the request`);
    } finally {
      setActingOn(null);
    }
  }

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Leave</h1>
          <p className="mt-1 text-sm text-slate-500">
            {canApprove ? "Review and decide leave requests across the team." : "Apply for leave and track its status."}
          </p>
        </div>
        {canApply && (
          <button
            type="button"
            onClick={() => setApplyOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
          >
            <CalendarCheck size={16} weight="bold" />
            Apply for Leave
          </button>
        )}
      </div>

      {error && <p className="mb-4 text-sm text-error">{error}</p>}

      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-card">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead className="bg-slate-50 text-xs font-semibold uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-4 py-3">Employee</th>
              <th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Dates</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Status</th>
              {canApprove && <th className="px-4 py-3">Actions</th>}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {requests === null ? (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center text-slate-400">
                  Loading&hellip;
                </td>
              </tr>
            ) : requests.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center text-slate-400">
                  No leave requests yet.
                </td>
              </tr>
            ) : (
              requests.map((r) => (
                <tr key={r.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <span className="flex items-center gap-3">
                      <Avatar name={r.employee_name} employeeId={r.employee_id} size={30} />
                      <span className="font-medium text-slate-800">{r.employee_name}</span>
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{LEAVE_TYPE_LABEL[r.leave_type]}</td>
                  <td className="px-4 py-3 text-slate-600">
                    {r.start_date === r.end_date ? formatDate(r.start_date) : `${formatDate(r.start_date)} – ${formatDate(r.end_date)}`}
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 text-slate-600" title={r.reason}>
                    {r.reason || "—"}
                    {r.has_document && (
                      <a
                        href={`${getBackendUrl()}/api/v1/leave/${r.id}/document`}
                        target="_blank"
                        rel="noreferrer"
                        className="ml-1.5 inline-flex align-middle text-brand-blue"
                        title="View attached document"
                      >
                        <Paperclip size={14} weight="bold" />
                      </a>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${STATUS_STYLES[r.status]}`}>
                      {r.status}
                    </span>
                    {r.status !== "PENDING" && r.decided_by_name && (
                      <p className="mt-0.5 text-[11px] text-slate-400">by {r.decided_by_name}</p>
                    )}
                  </td>
                  {canApprove && (
                    <td className="px-4 py-3">
                      {r.status === "PENDING" ? (
                        <div className="flex gap-2">
                          <button
                            type="button"
                            disabled={actingOn === r.id}
                            onClick={() => decide(r.id, "approve")}
                            className="flex items-center gap-1 rounded-lg bg-success px-2.5 py-1.5 text-xs font-semibold text-white hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <CheckCircle size={13} weight="bold" />
                            Approve
                          </button>
                          <button
                            type="button"
                            disabled={actingOn === r.id}
                            onClick={() => decide(r.id, "reject")}
                            className="flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-semibold text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <XCircle size={13} weight="bold" />
                            Reject
                          </button>
                        </div>
                      ) : (
                        <span className="text-xs text-slate-300">—</span>
                      )}
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {applyOpen && (
        <ApplyLeaveModal
          employees={employees}
          onClose={() => setApplyOpen(false)}
          onApplied={() => {
            setApplyOpen(false);
            load();
          }}
        />
      )}
    </div>
  );
}

function ApplyLeaveModal({
  employees,
  onClose,
  onApplied,
}: {
  employees: Employee[];
  onClose: () => void;
  onApplied: () => void;
}) {
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [leaveType, setLeaveType] = useState<LeaveType>("PAID");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [reason, setReason] = useState("");
  const [documentFile, setDocumentFile] = useState<File | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!employee || !startDate || !endDate) return;
    setSaving(true);
    setError(null);
    try {
      const formData = new FormData();
      formData.append("employee_id", String(employee.id));
      formData.append("leave_type", leaveType);
      formData.append("start_date", startDate);
      formData.append("end_date", endDate);
      formData.append("reason", reason);
      if (documentFile) formData.append("document", documentFile);
      await apiUpload("/api/v1/leave", formData);
      onApplied();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to submit the leave request");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4">
      <div className="max-h-[90vh] w-full max-w-md overflow-y-auto rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">Apply for Leave</h2>
          <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {!employee && <label className="mb-1 block text-xs font-semibold text-slate-600">Employee</label>}
          <EmployeeSearchPicker employees={employees} selected={employee} onSelect={setEmployee} autoFocus placeholder="Search your name…" />

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">Leave Type</label>
            <div className="flex gap-2">
              {(["PAID", "COMPOFF", "UNPAID"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setLeaveType(t)}
                  className={`flex-1 rounded-lg border px-3 py-2 text-xs font-semibold transition-colors ${
                    leaveType === t
                      ? "border-brand-blue bg-light-blue text-brand-blue"
                      : "border-slate-200 text-slate-600 hover:bg-slate-50"
                  }`}
                >
                  {LEAVE_TYPE_LABEL[t]}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs font-semibold text-slate-600">From</label>
              <DateField value={startDate} onChange={setStartDate} required className="input" />
            </div>
            <div>
              <label className="mb-1 block text-xs font-semibold text-slate-600">To</label>
              <DateField value={endDate} onChange={setEndDate} min={startDate || undefined} required className="input" />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">
              Reason <span className="font-normal text-slate-400">(optional)</span>
            </label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              className="input resize-none"
              placeholder="e.g. Family function"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-600">
              Supporting document <span className="font-normal text-slate-400">(optional)</span>
            </label>
            <input
              type="file"
              accept=".pdf,.jpg,.jpeg,.png,.webp"
              onChange={(e) => setDocumentFile(e.target.files?.[0] ?? null)}
              className="block w-full text-xs text-slate-500 file:mr-2 file:rounded-lg file:border-0 file:bg-light-blue file:px-3 file:py-1.5 file:text-xs file:font-semibold file:text-brand-blue"
            />
          </div>

          {error && <p className="text-sm text-error">{error}</p>}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={!employee || !startDate || !endDate || saving}
              className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? "Submitting…" : "Submit Request"}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
