"use client";

import { use, useEffect, useState } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import { EmployeeForm } from "../../_components/EmployeeForm";
import type { Employee } from "../../_components/types";

export default function EditEmployeePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Employee>(`/api/v1/employees/${id}`)
      .then(setEmployee)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employee"));
  }, [id]);

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
