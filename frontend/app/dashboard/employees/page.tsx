"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { MagnifyingGlass, Plus, WarningCircle } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { Avatar } from "@/components/Avatar";
import { EmployeeStatusBadge } from "./_components/EmployeeStatusBadge";
import type { Employee } from "./_components/types";
import { formatDate } from "../_components/my/types";
import { useSession } from "../_components/SessionProvider";

function matchesSearch(e: Employee, needle: string): boolean {
  const haystack = [e.full_name, e.employee_code].join(" ").toLowerCase();
  return haystack.includes(needle);
}

export default function EmployeesPage() {
  const { can } = useSession();
  const canManage = can("employees.manage");
  const [employees, setEmployees] = useState<Employee[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Seeded from ?q=... when arriving from the topbar's "View all results" link.
  const [search, setSearch] = useState("");

  useEffect(() => {
    const q = new URLSearchParams(window.location.search).get("q");
    if (q) setSearch(q);
  }, []);

  useEffect(() => {
    // No department/position/designation/status filters — small enough a
    // roster (a handful of employees) that the list itself is the filter.
    // Omitting `status` entirely already means "active employees only",
    // same as the removed filter's own default (see the backend's List).
    apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
      .then((data) => setEmployees(data.employees ?? []))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employees"));
  }, []);

  const missingDocsCount = (employees ?? []).filter((e) => !e.documents_complete).length;
  const needle = search.trim().toLowerCase();
  const visibleEmployees = (employees ?? []).filter((e) => !needle || matchesSearch(e, needle));

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Employees</h1>
          <p className="mt-1 text-sm text-slate-500">Manage employee records across your organization.</p>
        </div>
        {canManage && (
          <Link
            href="/dashboard/employees/new"
            className="flex items-center gap-2 rounded-lg bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue-dark"
          >
            <Plus size={16} weight="bold" />
            Add Employee
          </Link>
        )}
      </div>

      <div className="mb-4 flex flex-wrap gap-3">
        <div className="relative min-w-[200px] max-w-[260px] flex-1">
          <MagnifyingGlass size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            type="text"
            placeholder="Search name or employee code…"
            className="input w-full pl-8"
          />
        </div>
      </div>

      {error && <p className="mb-3 text-sm text-error">{error}</p>}

      {missingDocsCount > 0 && (
        <div className="mb-4 flex items-center gap-2.5 rounded-xl border border-warning/30 bg-warning-bg px-4 py-3">
          <WarningCircle size={20} className="shrink-0 text-warning" weight="bold" />
          <p className="text-sm text-slate-700">
            <span className="font-semibold">
              {missingDocsCount} employee{missingDocsCount === 1 ? "" : "s"}
            </span>{" "}
            {missingDocsCount === 1 ? "is" : "are"} missing one or more required documents (Offer Letter, Aadhar
            Card, PAN Card, or Marksheet).
          </p>
        </div>
      )}

      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-card">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-50 text-xs font-semibold uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-4 py-3">Employee</th>
              <th className="px-4 py-3">Department</th>
              <th className="px-4 py-3">Position</th>
              <th className="px-4 py-3">Designation</th>
              <th className="px-4 py-3">Shift</th>
              <th className="px-4 py-3">Joined</th>
              <th className="px-4 py-3">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {employees === null ? (
              <tr>
                <td colSpan={8} className="px-4 py-6 text-center text-slate-400">
                  Loading&hellip;
                </td>
              </tr>
            ) : visibleEmployees.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-6 text-center text-slate-400">
                  {search.trim() ? "No employees match your search." : "No employees yet."}
                </td>
              </tr>
            ) : (
              visibleEmployees.map((emp) => (
                <tr key={emp.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <Link href={`/dashboard/employees/${emp.id}`} className="flex items-center gap-3">
                      <Avatar name={emp.full_name} employeeId={emp.id} size={34} />
                      <span>
                        <span className="block font-medium text-slate-800">{emp.full_name}</span>
                        <span className="block text-xs text-slate-400">{emp.employee_code}</span>
                      </span>
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{emp.department_name ?? "—"}</td>
                  <td className="px-4 py-3 text-slate-600">{emp.position_title ?? "—"}</td>
                  <td className="px-4 py-3 text-slate-600">{emp.designation_title ?? "—"}</td>
                  <td className="px-4 py-3 text-slate-600">{emp.shift_name ?? "—"}</td>
                  <td className="px-4 py-3 text-slate-600">{formatDate(emp.joining_date)}</td>
                  <td className="px-4 py-3">
                    <EmployeeStatusBadge status={emp.status} />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
