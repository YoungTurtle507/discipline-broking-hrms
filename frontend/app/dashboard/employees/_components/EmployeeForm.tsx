"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { CircleNotch } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { DateField } from "../../_components/DateField";
import type { Department, Designation, Employee, Position, Shift } from "./types";

// Standard "name@domain.tld" shape — matches how a gmail.com address (or any
// other real address) is formatted, without being restricted to one provider.
const EMAIL_PATTERN = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/;
const PHONE_PATTERN = /^[0-9]{10}$/;

export function EmployeeForm({ employee }: { employee?: Employee }) {
  const router = useRouter();
  const isEdit = !!employee;

  const [departments, setDepartments] = useState<Department[]>([]);
  const [allDesignations, setAllDesignations] = useState<Designation[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [shifts, setShifts] = useState<Shift[]>([]);

  const [fullName, setFullName] = useState(employee?.full_name ?? "");
  const [email, setEmail] = useState(employee?.email ?? "");
  const [phone, setPhone] = useState(employee?.phone ?? "");
  const [joiningDate, setJoiningDate] = useState(employee?.joining_date ?? "");
  const [dateOfBirth, setDateOfBirth] = useState(employee?.date_of_birth ?? "");
  const [address, setAddress] = useState(employee?.address ?? "");
  const [departmentId, setDepartmentId] = useState(employee?.department_id?.toString() ?? "");
  const [designationId, setDesignationId] = useState(employee?.designation_id?.toString() ?? "");
  const [positionId, setPositionId] = useState(employee?.position_id?.toString() ?? "");
  const [shiftId, setShiftId] = useState(employee?.shift_id?.toString() ?? "");

  // Designation choices are whatever belongs to the selected department — a
  // Coding designation is never offered once Dealer is picked, and vice versa.
  const designations = departmentId
    ? allDesignations.filter((d) => d.department_id === Number(departmentId))
    : [];

  function handleDepartmentChange(value: string) {
    setDepartmentId(value);
    // The previously chosen designation may not exist in the new department,
    // so it's cleared rather than silently carried over to the wrong one.
    setDesignationId("");
  }

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<{ departments: Department[] | null }>("/api/v1/departments").then((d) =>
      setDepartments((d.departments ?? []).filter((x) => x.is_active))
    );
    apiFetch<{ designations: Designation[] | null }>("/api/v1/designations").then((d) =>
      setAllDesignations((d.designations ?? []).filter((x) => x.is_active))
    );
    apiFetch<{ positions: Position[] | null }>("/api/v1/positions").then((d) =>
      setPositions((d.positions ?? []).filter((x) => x.is_active))
    );
    apiFetch<{ shifts: Shift[] | null }>("/api/v1/shifts").then((d) => {
      const active = (d.shifts ?? []).filter((x) => x.is_active);
      setShifts(active);
      // New employees default to the first shift rather than starting blank —
      // there is always at least one shift once the company is set up at all.
      setShiftId((prev) => prev || (active[0] ? String(active[0].id) : ""));
    });
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);

    const payload = {
      full_name: fullName,
      email,
      phone,
      joining_date: joiningDate,
      date_of_birth: dateOfBirth,
      address,
      department_id: departmentId ? Number(departmentId) : null,
      designation_id: designationId ? Number(designationId) : null,
      position_id: positionId ? Number(positionId) : null,
      shift_id: shiftId ? Number(shiftId) : null,
    };

    try {
      const saved = isEdit
        ? await apiFetch<Employee>(`/api/v1/employees/${employee!.id}`, {
            method: "PUT",
            body: JSON.stringify(payload),
          })
        : await apiFetch<Employee>("/api/v1/employees", {
            method: "POST",
            body: JSON.stringify(payload),
          });
      router.push(isEdit ? `/dashboard/employees/${saved.id}` : `/dashboard/employees/${saved.id}/face`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save employee");
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="rounded-xl border border-slate-200 bg-white p-6 shadow-card">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Full Name" required>
          <input
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            required
            className="input"
          />
        </Field>
        <Field label="Email">
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            pattern={EMAIL_PATTERN.source}
            title="Enter a properly formatted email address, e.g. name@gmail.com"
            placeholder="name@gmail.com"
            className="input"
          />
        </Field>
        <Field label="Phone" required>
          <input
            type="tel"
            inputMode="numeric"
            value={phone}
            onChange={(e) => setPhone(e.target.value.replace(/[^0-9]/g, "").slice(0, 10))}
            required
            pattern={PHONE_PATTERN.source}
            maxLength={10}
            title="Enter exactly 10 digits, no spaces or country code"
            placeholder="9876543210"
            className="input"
          />
        </Field>
        <Field label="Joining Date" required>
          <DateField value={joiningDate} onChange={setJoiningDate} required className="input" />
        </Field>
        <Field label="Date of Birth">
          <DateField value={dateOfBirth} onChange={setDateOfBirth} max={new Date().toISOString().slice(0, 10)} className="input" />
        </Field>
        <Field label="Department">
          <select value={departmentId} onChange={(e) => handleDepartmentChange(e.target.value)} className="input">
            <option value="">&mdash; None &mdash;</option>
            {departments.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Position">
          <select value={positionId} onChange={(e) => setPositionId(e.target.value)} className="input">
            <option value="">&mdash; None &mdash;</option>
            {positions.map((p) => (
              <option key={p.id} value={p.id}>
                {p.title}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Designation">
          <select
            value={designationId}
            onChange={(e) => setDesignationId(e.target.value)}
            disabled={!departmentId}
            className="input disabled:cursor-not-allowed disabled:opacity-60"
          >
            <option value="">{departmentId ? "— None —" : "Select a department first"}</option>
            {designations.map((d) => (
              <option key={d.id} value={d.id}>
                {d.title}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Shift" required>
          <select value={shiftId} onChange={(e) => setShiftId(e.target.value)} required className="input">
            <option value="" disabled>
              Select a shift
            </option>
            {shifts.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.start_time.slice(0, 5)}–{s.end_time.slice(0, 5)})
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-slate-400">
            Sets which shift timing this employee&apos;s late arrival and half-day cutoffs are measured against.
          </span>
        </Field>
        <Field label="Address">
          <textarea
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            rows={2}
            className="input resize-none"
          />
        </Field>
      </div>

      {error && <p className="mt-4 text-sm text-error">{error}</p>}

      <div className="mt-6 flex gap-3">
        <button
          type="submit"
          disabled={saving}
          className="flex items-center gap-2 rounded-lg bg-brand-blue px-5 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:opacity-70"
        >
          {saving && <CircleNotch size={16} className="animate-spin" />}
          {isEdit ? "Save Changes" : "Add Employee"}
        </button>
        <button
          type="button"
          onClick={() => router.back()}
          className="rounded-xl border border-slate-200 px-5 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

function Field({ label, required, children }: { label: string; required?: boolean; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-semibold text-slate-600">
        {label}
        {required && <span className="text-error"> *</span>}
      </span>
      {children}
    </label>
  );
}
