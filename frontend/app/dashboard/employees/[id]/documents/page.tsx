"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Employee } from "../../_components/types";
import { DocumentsCard } from "../../_components/DocumentsCard";

export default function EmployeeDocumentsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Employee>(`/api/v1/employees/${id}`)
      .then(setEmployee)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load employee"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (error && !employee) return <p className="text-sm text-error">{error}</p>;
  if (!employee) return <p className="text-sm text-slate-400">Loading&hellip;</p>;

  return (
    <div>
      <Link
        href={`/dashboard/employees/${id}`}
        className="mb-4 inline-flex items-center gap-2 text-sm font-medium text-slate-500 hover:text-slate-700"
      >
        <ArrowLeft size={16} />
        Back to Profile
      </Link>

      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Documents</h1>
        <p className="mt-1 text-sm text-slate-500">
          Offer Letter, Aadhar Card, PAN Card, and Marksheet on file for {employee.full_name}.
        </p>
      </div>

      <DocumentsCard employeeId={employee.id} />
    </div>
  );
}
