"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, PencilSimple, EnvelopeSimple, Phone, Calendar, Buildings, IdentificationCard, Clock, HourglassMedium, Key, CircleNotch, FileText, MapPin, Camera, X } from "@phosphor-icons/react";
import { apiFetch, apiUpload, getBackendUrl, ApiError } from "@/lib/api";
import { Avatar } from "@/components/Avatar";
import { EmployeeStatusBadge } from "../_components/EmployeeStatusBadge";
import { STATUS_OPTIONS, DEFAULT_NOTICE_PERIOD_DAYS, type Employee } from "../_components/types";
import { AttendanceHistory } from "./_components/AttendanceHistory";
import { DOCUMENT_SLOTS } from "../_components/DocumentsCard";
import { formatDate } from "../../_components/my/types";
import { DateField } from "../../_components/DateField";
import { useSession } from "../../_components/SessionProvider";

function defaultNoticeDate(): string {
  const d = new Date();
  d.setDate(d.getDate() + DEFAULT_NOTICE_PERIOD_DAYS);
  return d.toISOString().slice(0, 10);
}

function daysRemaining(lastWorkingDay: string): number {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const target = new Date(lastWorkingDay + "T00:00:00");
  return Math.round((target.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));
}

export default function EmployeeProfilePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { can } = useSession();
  const canManage = can("employees.manage");
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [updatingStatus, setUpdatingStatus] = useState(false);
  const [noticeDate, setNoticeDate] = useState(defaultNoticeDate());
  const [pickingNotice, setPickingNotice] = useState(false);

  // Issuing a login for an employee who doesn't have one yet. Employees added
  // before they needed to sign in (or created without a password) get their
  // credentials here. Resets afterwards happen in Settings > Users.
  const [issuingLogin, setIssuingLogin] = useState(false);
  const [loginPassword, setLoginPassword] = useState("");
  const [loginSaving, setLoginSaving] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);

  const [selfCheckinSaving, setSelfCheckinSaving] = useState(false);
  const [selfCheckinError, setSelfCheckinError] = useState<string | null>(null);

  // Manually setting/replacing the avatar photo directly, independent of
  // face registration — saved to the exact same slot a FRONT sample's
  // capture would use, so whichever happened most recently is simply what
  // shows. A full reload after a successful upload is the simplest reliable
  // way to bust the browser's cache of the old photo everywhere it's shown.
  const [uploadingPhoto, setUploadingPhoto] = useState(false);
  const [photoError, setPhotoError] = useState<string | null>(null);
  const [photoViewerOpen, setPhotoViewerOpen] = useState(false);
  const [photoViewerFailed, setPhotoViewerFailed] = useState(false);

  async function handlePhotoSelected(file: File) {
    setUploadingPhoto(true);
    setPhotoError(null);
    try {
      const formData = new FormData();
      formData.append("file", file);
      await apiUpload(`/api/v1/employees/${id}/photo`, formData);
      window.location.reload();
    } catch (err) {
      setPhotoError(err instanceof ApiError ? err.message : "Failed to upload photo");
      setUploadingPhoto(false);
    }
  }

  // How many of the required document slots (see DOCUMENT_SLOTS) are still
  // missing — null while unknown, so the flag only ever appears once we
  // actually know there's something missing, never as a false-positive flash.
  const [missingDocs, setMissingDocs] = useState<number | null>(null);

  async function load() {
    try {
      const data = await apiFetch<Employee>(`/api/v1/employees/${id}`);
      setEmployee(data);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load employee");
    }
    try {
      const docs = await apiFetch<{ documents: Record<string, unknown> | null }>(`/api/v1/employees/${id}/documents`);
      const uploaded = Object.keys(docs.documents ?? {}).length;
      setMissingDocs(Math.max(0, DOCUMENT_SLOTS.length - uploaded));
    } catch {
      // Non-critical — the Documents button just won't show a flag.
    }
  }

  async function handleIssueLogin(e: React.FormEvent) {
    e.preventDefault();
    setLoginSaving(true);
    setLoginError(null);
    try {
      await apiFetch(`/api/v1/employees/${id}/login`, {
        method: "POST",
        body: JSON.stringify({ password: loginPassword }),
      });
      setIssuingLogin(false);
      setLoginPassword("");
      await load();
    } catch (err) {
      setLoginError(err instanceof ApiError ? err.message : "Failed to create login account");
    } finally {
      setLoginSaving(false);
    }
  }

  async function toggleSelfCheckin(enabled: boolean) {
    setSelfCheckinSaving(true);
    setSelfCheckinError(null);
    try {
      await apiFetch(`/api/v1/employees/${id}/self-checkin`, {
        method: "PATCH",
        body: JSON.stringify({ enabled }),
      });
      await load();
    } catch (err) {
      setSelfCheckinError(err instanceof ApiError ? err.message : "Failed to update self check-in");
    } finally {
      setSelfCheckinSaving(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function changeStatus(status: string, lastWorkingDay?: string) {
    setUpdatingStatus(true);
    try {
      await apiFetch(`/api/v1/employees/${id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status, last_working_day: lastWorkingDay ?? null }),
      });
      setPickingNotice(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update status");
    } finally {
      setUpdatingStatus(false);
    }
  }

  function selectStatus(s: string) {
    if (s === "ON_NOTICE") {
      setNoticeDate(defaultNoticeDate());
      setPickingNotice(true);
      return;
    }
    changeStatus(s);
  }

  if (error) return <p className="text-sm text-error">{error}</p>;
  if (!employee) return <p className="text-sm text-slate-400">Loading&hellip;</p>;

  return (
    <div>
      <Link
        href="/dashboard/employees"
        className="mb-4 inline-flex items-center gap-2 text-sm font-medium text-slate-500 hover:text-slate-700"
      >
        <ArrowLeft size={16} />
        Back to Employees
      </Link>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-center gap-4">
          <div className="relative">
            <button
              type="button"
              onClick={() => {
                setPhotoViewerFailed(false);
                setPhotoViewerOpen(true);
              }}
              className="cursor-pointer outline-none ring-brand-blue/50 transition hover:opacity-90 focus-visible:ring-2"
              title="View profile photo"
            >
              <Avatar name={employee.full_name} employeeId={employee.id} size={56} />
            </button>
            {canManage && (
              <label
                className="absolute -bottom-1 -right-1 flex h-6 w-6 cursor-pointer items-center justify-center rounded-full border border-white bg-brand-blue text-white hover:bg-brand-blue-dark"
                title="Upload profile photo"
                onClick={(e) => e.stopPropagation()}
              >
                {uploadingPhoto ? <CircleNotch size={12} className="animate-spin" /> : <Camera size={12} weight="fill" />}
                <input
                  type="file"
                  accept="image/jpeg,image/png"
                  className="hidden"
                  disabled={uploadingPhoto}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) handlePhotoSelected(file);
                  }}
                />
              </label>
            )}
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-900">{employee.full_name}</h1>
            <p className="text-sm text-slate-500">
              {employee.employee_code} &middot; {employee.designation_title ?? "No designation"}
            </p>
            {photoError && <p className="mt-1 text-xs text-error">{photoError}</p>}
          </div>
        </div>
        <div className="flex items-center gap-3">
          <EmployeeStatusBadge status={employee.status} />
          <Link
            href={`/dashboard/employees/${employee.id}/documents`}
            className="relative flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50"
          >
            <FileText size={16} />
            Documents
            {!!missingDocs && (
              <span
                className="absolute -right-1.5 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-error px-1 text-[10px] font-bold text-white"
                title={`${missingDocs} document${missingDocs === 1 ? "" : "s"} missing`}
              >
                {missingDocs}
              </span>
            )}
          </Link>
          {canManage && (
            <Link
              href={`/dashboard/employees/${employee.id}/edit`}
              className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              <PencilSimple size={16} />
              Edit
            </Link>
          )}
        </div>
      </div>

      <div className={`grid grid-cols-1 gap-6 ${canManage ? "lg:grid-cols-3" : ""}`}>
        <div className={`rounded-xl border border-slate-200 bg-white p-5 shadow-card ${canManage ? "lg:col-span-2" : ""}`}>
          <h2 className="mb-4 text-base font-semibold text-slate-900">Details</h2>
          <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <DetailRow icon={EnvelopeSimple} label="Email" value={employee.email ?? "—"} />
            <DetailRow icon={Phone} label="Phone" value={employee.phone || "—"} />
            <DetailRow icon={Calendar} label="Joining Date" value={formatDate(employee.joining_date)} />
            <DetailRow icon={Buildings} label="Department" value={employee.department_name ?? "—"} />
            <DetailRow icon={IdentificationCard} label="Position" value={employee.position_title ?? "—"} />
            <DetailRow icon={IdentificationCard} label="Designation" value={employee.designation_title ?? "—"} />
            <DetailRow icon={Clock} label="Shift" value={employee.shift_name ?? "—"} />
            <DetailRow icon={Calendar} label="Date of Birth" value={employee.date_of_birth ? formatDate(employee.date_of_birth) : "—"} />
            <DetailRow icon={MapPin} label="Address" value={employee.address || "—"} />
          </dl>

          {employee.status === "ON_NOTICE" && employee.last_working_day && (
            <div className="mt-5 flex items-center gap-3 rounded-lg bg-warning-bg px-4 py-3">
              <HourglassMedium size={20} className="shrink-0 text-warning" weight="bold" />
              <p className="text-sm text-slate-700">
                On notice &middot; last working day{" "}
                <span className="font-semibold">{formatDate(employee.last_working_day)}</span>{" "}
                &middot;{" "}
                <span className="font-semibold">
                  {Math.max(0, daysRemaining(employee.last_working_day))} day
                  {daysRemaining(employee.last_working_day) === 1 ? "" : "s"} remaining
                </span>
                . Will auto-move to Resigned once the date passes.
              </p>
            </div>
          )}

          <div className="mt-5 border-t border-slate-100 pt-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <h3 className="text-sm font-semibold text-slate-800">System Login</h3>
                <p className="text-xs text-slate-500">
                  {employee.user_id
                    ? `Signs in as ${employee.login_username ?? employee.employee_code}. Reset the password from Settings > Users.`
                    : "No login yet — this employee can't sign in to the system."}
                </p>
              </div>
              {employee.user_id ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-success-bg px-2.5 py-1 text-xs font-semibold text-success">
                  <Key size={13} />
                  {employee.login_username ?? employee.employee_code}
                </span>
              ) : (
                canManage &&
                !issuingLogin && (
                  <button
                    type="button"
                    onClick={() => {
                      setIssuingLogin(true);
                      setLoginError(null);
                    }}
                    className="flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50"
                  >
                    <Key size={14} />
                    Create login
                  </button>
                )
              )}
            </div>

            {canManage && issuingLogin && !employee.user_id && (
              <form onSubmit={handleIssueLogin} className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
                <div className="flex-1 sm:max-w-xs">
                  <span className="mb-1.5 block text-xs font-semibold text-slate-600">
                    Password for {employee.employee_code}
                  </span>
                  <input
                    value={loginPassword}
                    onChange={(e) => setLoginPassword(e.target.value)}
                    type="password"
                    required
                    minLength={8}
                    autoComplete="new-password"
                    placeholder="At least 8 characters"
                    className="input"
                  />
                </div>
                <div className="flex gap-2">
                  <button
                    type="submit"
                    disabled={loginSaving}
                    className="flex items-center gap-2 rounded-xl bg-navy px-4 py-2 text-sm font-semibold text-white hover:bg-navy-light disabled:opacity-70"
                  >
                    {loginSaving && <CircleNotch size={14} className="animate-spin" />}
                    Create
                  </button>
                  <button
                    type="button"
                    onClick={() => setIssuingLogin(false)}
                    className="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50"
                  >
                    Cancel
                  </button>
                </div>
                <p className="basis-full text-xs text-slate-500">
                  Username will be <span className="font-semibold">{employee.employee_code}</span>. Tell them this
                  password &mdash; it can be reset later but never viewed.
                </p>
                {loginError && <p className="basis-full text-xs text-error">{loginError}</p>}
              </form>
            )}
          </div>

          {canManage && employee.user_id && (
            <div className="mt-5 border-t border-slate-100 pt-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h3 className="text-sm font-semibold text-slate-800">Remote Self Check-In</h3>
                  <p className="text-xs text-slate-500">
                    {employee.remote_self_checkin
                      ? "This employee can mark their own attendance from their dashboard — no face scan needed."
                      : "Off — this employee must clock in by face scan like everyone else."}
                  </p>
                </div>
                <button
                  type="button"
                  disabled={selfCheckinSaving}
                  onClick={() => toggleSelfCheckin(!employee.remote_self_checkin)}
                  className={`flex items-center gap-2 rounded-xl border px-3 py-2 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-60 ${
                    employee.remote_self_checkin
                      ? "border-success/30 bg-success-bg text-success hover:bg-success-bg/70"
                      : "border-slate-200 text-slate-700 hover:bg-slate-50"
                  }`}
                >
                  {selfCheckinSaving && <CircleNotch size={14} className="animate-spin" />}
                  {employee.remote_self_checkin ? "Enabled — turn off" : "Turn on"}
                </button>
              </div>
              {selfCheckinError && <p className="mt-2 text-xs text-error">{selfCheckinError}</p>}
            </div>
          )}
        </div>

        {canManage && (
        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
          <h2 className="mb-4 text-base font-semibold text-slate-900">Change Status</h2>
          <div className="flex flex-col gap-2">
            {STATUS_OPTIONS.map((s) => (
              <button
                key={s}
                type="button"
                disabled={updatingStatus || s === employee.status}
                onClick={() => selectStatus(s)}
                className={`rounded-lg border px-3 py-2 text-left text-sm font-medium transition-colors ${
                  s === employee.status
                    ? "border-brand-blue bg-light-blue text-brand-blue"
                    : "border-slate-200 text-slate-600 hover:bg-slate-50"
                } disabled:cursor-default`}
              >
                {s.replace("_", " ")}
              </button>
            ))}
          </div>

          {pickingNotice && (
            <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-3">
              <label className="mb-1.5 block text-xs font-semibold text-slate-600">
                Last Working Day
              </label>
              <DateField
                value={noticeDate}
                onChange={setNoticeDate}
                min={new Date().toISOString().slice(0, 10)}
                className="input"
              />
              <div className="mt-3 flex gap-2">
                <button
                  type="button"
                  disabled={updatingStatus || !noticeDate}
                  onClick={() => changeStatus("ON_NOTICE", noticeDate)}
                  className="flex-1 rounded-lg bg-brand-blue px-3 py-2 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:opacity-70"
                >
                  Confirm Notice
                </button>
                <button
                  type="button"
                  onClick={() => setPickingNotice(false)}
                  className="rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 hover:bg-white"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}

          <p className="mt-3 text-xs text-slate-400">
            Employees are never deleted — only their status changes.
          </p>
        </div>
        )}
      </div>

      <div className="mt-6">
        <AttendanceHistory employeeId={id} />
      </div>

      {photoViewerOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/80 p-4"
          onClick={() => setPhotoViewerOpen(false)}
        >
          <button
            type="button"
            onClick={() => setPhotoViewerOpen(false)}
            className="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-white hover:bg-white/20"
            aria-label="Close"
          >
            <X size={20} />
          </button>
          <div className="max-w-[90vw]" onClick={(e) => e.stopPropagation()}>
            {photoViewerFailed ? (
              <div className="flex h-64 w-64 flex-col items-center justify-center gap-2 rounded-xl bg-slate-800 text-slate-300">
                <Camera size={32} />
                <p className="text-sm">No profile photo uploaded yet</p>
              </div>
            ) : (
              <img
                src={`${getBackendUrl()}/api/v1/employees/${employee.id}/face-photo`}
                alt={employee.full_name}
                onError={() => setPhotoViewerFailed(true)}
                className="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl"
              />
            )}
            <p className="mt-3 text-center text-sm font-medium text-white">{employee.full_name}</p>
          </div>
        </div>
      )}
    </div>
  );
}

function DetailRow({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof EnvelopeSimple;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-start gap-3">
      <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-light-blue text-brand-blue">
        <Icon size={16} weight="bold" />
      </span>
      <div className="min-w-0">
        <dt className="text-xs text-slate-400">{label}</dt>
        <dd className="truncate text-sm font-medium text-slate-800">{value}</dd>
      </div>
    </div>
  );
}
