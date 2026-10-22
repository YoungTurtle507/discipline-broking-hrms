"use client";

import { use, useEffect, useState } from "react";
import { LockSimple } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { EmployeeForm } from "../../_components/EmployeeForm";
import { useSession } from "../../../_components/SessionProvider";
import type { Employee } from "../../_components/types";

export default function EditEmployeePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { can, loading: sessionLoading } = useSession();
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Employee>(`/api/v1/employees/${id}`)
      .then(setEmployee)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employee"));
  }, [id]);

  if (sessionLoading) return <p className="text-sm text-slate-400">Loading&hellip;</p>;
  if (!can("employees.manage")) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-slate-200 bg-white py-16 text-center">
        <LockSimple size={28} className="text-slate-300" />
        <p className="text-sm font-semibold text-slate-700">View-only access</p>
        <p className="max-w-sm text-sm text-slate-400">Your login can view employee records but can&apos;t edit them.</p>
      </div>
    );
  }
  if (error) return <p className="text-sm text-error">{error}</p>;
  if (!employee) return <p className="text-sm text-slate-400">Loading&hellip;</p>;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Edit Employee</h1>
        <p className="mt-1 text-sm text-slate-500">Update {employee.full_name}&apos;s record.</p>
      </div>
      <EmployeeForm employee={employee} />
    </div>
  );
}
