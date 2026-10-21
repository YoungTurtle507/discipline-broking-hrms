import { EmployeeForm } from "../_components/EmployeeForm";

export default function NewEmployeePage() {
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
