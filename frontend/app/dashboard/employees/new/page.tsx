"use client";

import { LockSimple } from "@phosphor-icons/react";
import { EmployeeForm } from "../_components/EmployeeForm";
import { useSession } from "../../_components/SessionProvider";

export default function NewEmployeePage() {
  const { can, loading } = useSession();

  if (loading) {
    return <p className="text-sm text-slate-400">Loading&hellip;</p>;
  }
  if (!can("employees.manage")) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-slate-200 bg-white py-16 text-center">
        <LockSimple size={28} className="text-slate-300" />
        <p className="text-sm font-semibold text-slate-700">View-only access</p>
        <p className="max-w-sm text-sm text-slate-400">Your login can view employee records but can&apos;t add new ones.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Add Employee</h1>
        <p className="mt-1 text-sm text-slate-500">Create a new employee record.</p>
      </div>
      <EmployeeForm />
    </div>
  );
}
