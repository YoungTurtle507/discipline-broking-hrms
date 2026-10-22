"use client";

import { useEffect, useState } from "react";
import { Buildings, IdentificationCard, Briefcase, Plus, PencilSimple, CheckCircle, XCircle, LockSimple } from "@phosphor-icons/react";
import { apiFetch, ApiError } from "@/lib/api";
import { useSession } from "../_components/SessionProvider";
import type { Department, Position, Designation } from "../employees/_components/types";

type Tab = "departments" | "positions" | "designations";

// The sidebar already hides this link from anyone but a Super Admin, but
// departments/positions/designations are deliberately readable by ANY signed-in
// user at the API level (see backend's `requireAuth`-only GET routes) — only the
// writes are `requireRole("SUPER_ADMIN")`. Without this guard, a non-Super-Admin
// login (e.g. the shared kiosk account) that reaches this URL directly would see
// a page full of Add/Edit/activate controls that silently 403 the moment they're
// used, rather than being told up front that this page isn't for them.
export default function OrganizationPage() {
  const { me, loading: sessionLoading } = useSession();
  const isSuperAdmin = me?.roles.includes("SUPER_ADMIN") ?? false;

  if (sessionLoading) {
    return <p className="text-sm text-slate-400">Loading&hellip;</p>;
  }
  if (!isSuperAdmin) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-slate-200 bg-white py-16 text-center">
        <LockSimple size={28} className="text-slate-300" />
        <p className="text-sm font-semibold text-slate-700">Super Admin access required</p>
        <p className="max-w-sm text-sm text-slate-400">
          Organization settings (departments, positions, designations) can only be changed by a Super Admin.
        </p>
      </div>
    );
  }

  return <OrganizationManager />;
}

function OrganizationManager() {
  const [tab, setTab] = useState<Tab>("departments");
  const [departments, setDepartments] = useState<Department[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [designations, setDesignations] = useState<Designation[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function loadAll() {
    try {
      const [d, p, de] = await Promise.all([
        apiFetch<{ departments: Department[] | null }>("/api/v1/departments"),
        apiFetch<{ positions: Position[] | null }>("/api/v1/positions"),
        apiFetch<{ designations: Designation[] | null }>("/api/v1/designations"),
      ]);
      setDepartments(d.departments ?? []);
      setPositions(p.positions ?? []);
      setDesignations(de.designations ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load organization data");
    }
  }

  useEffect(() => {
    loadAll();
  }, []);

  const TABS: { key: Tab; label: string; icon: typeof Buildings }[] = [
    { key: "departments", label: "Departments", icon: Buildings },
    { key: "positions", label: "Positions", icon: Briefcase },
    { key: "designations", label: "Designations", icon: IdentificationCard },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Organization</h1>
        <p className="mt-1 text-sm text-slate-500">Manage departments, positions, and designations.</p>
      </div>

      {error && <p className="mb-4 text-sm text-error">{error}</p>}

      <div className="mb-5 flex gap-1 border-b border-slate-200">
        {TABS.map(({ key, label, icon: TabIcon }) => (
          <button
            key={key}
            type="button"
            onClick={() => setTab(key)}
            className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-semibold transition-colors ${
              tab === key ? "border-brand-blue text-brand-blue" : "border-transparent text-slate-500 hover:text-slate-800"
            }`}
          >
            <TabIcon size={16} />
            {label}
          </button>
        ))}
      </div>

      {tab === "departments" && <DepartmentsPanel items={departments} onChanged={loadAll} />}
      {tab === "positions" && <PositionsPanel items={positions} onChanged={loadAll} />}
      {tab === "designations" && <DesignationsPanel items={designations} departments={departments} onChanged={loadAll} />}
    </div>
  );
}

// Shared row chrome: name/title, optional description, active badge, edit
// button, active/inactive toggle — identical across all three tabs, just
// fed different data and callbacks.
function EntityList({
  rows,
  onEdit,
  onToggleActive,
}: {
  rows: { id: number; primary: string; secondary?: string; isActive: boolean }[];
  onEdit: (id: number) => void;
  onToggleActive: (id: number, next: boolean) => void;
}) {
  if (rows.length === 0) {
    return <p className="py-6 text-center text-sm text-slate-400">Nothing here yet.</p>;
  }
  return (
    <div className="divide-y divide-slate-100">
      {rows.map((row) => (
        <div key={row.id} className="flex items-center gap-3 py-3">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-slate-800">{row.primary}</p>
            {row.secondary && <p className="truncate text-xs text-slate-400">{row.secondary}</p>}
          </div>
          <span
            className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${
              row.isActive ? "bg-success-bg text-success" : "bg-slate-100 text-slate-500"
            }`}
          >
            {row.isActive ? "Active" : "Inactive"}
          </span>
          <button
            type="button"
            onClick={() => onEdit(row.id)}
            className="shrink-0 rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50"
            title="Edit"
          >
            <PencilSimple size={14} />
          </button>
          <button
            type="button"
            onClick={() => onToggleActive(row.id, !row.isActive)}
            className="shrink-0 rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50"
            title={row.isActive ? "Deactivate" : "Activate"}
          >
            {row.isActive ? <XCircle size={14} /> : <CheckCircle size={14} />}
          </button>
        </div>
      ))}
    </div>
  );
}

function PanelShell({
  addLabel,
  onAddClick,
  children,
}: {
  addLabel: string;
  onAddClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
      <div className="mb-2 flex justify-end">
        <button
          type="button"
          onClick={onAddClick}
          className="flex items-center gap-2 rounded-lg bg-brand-blue px-3 py-2 text-xs font-semibold text-white hover:bg-brand-blue-dark"
        >
          <Plus size={14} weight="bold" />
          {addLabel}
        </button>
      </div>
      {children}
    </div>
  );
}

function DepartmentsPanel({ items, onChanged }: { items: Department[]; onChanged: () => void }) {
  const [editing, setEditing] = useState<Department | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function save(name: string, description: string) {
    try {
      if (editing === "new") {
        await apiFetch("/api/v1/departments", { method: "POST", body: JSON.stringify({ name, description }) });
      } else if (editing) {
        await apiFetch(`/api/v1/departments/${editing.id}`, { method: "PUT", body: JSON.stringify({ name, description }) });
      }
      setEditing(null);
      setError(null);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save department");
    }
  }

  async function toggleActive(id: number, next: boolean) {
    try {
      await apiFetch(`/api/v1/departments/${id}/active`, { method: "PATCH", body: JSON.stringify({ is_active: next }) });
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update department");
    }
  }

  return (
    <PanelShell addLabel="Add Department" onAddClick={() => setEditing("new")}>
      {error && <p className="mb-3 text-sm text-error">{error}</p>}
      <EntityList
        rows={items.map((d) => ({ id: d.id, primary: d.name, secondary: d.description, isActive: d.is_active }))}
        onEdit={(id) => setEditing(items.find((d) => d.id === id) ?? null)}
        onToggleActive={toggleActive}
      />
      {editing && (
        <NameDescriptionForm
          title={editing === "new" ? "Add Department" : "Edit Department"}
          nameLabel="Department name"
          initialName={editing === "new" ? "" : editing.name}
          initialDescription={editing === "new" ? "" : editing.description}
          onCancel={() => setEditing(null)}
          onSave={save}
        />
      )}
    </PanelShell>
  );
}

function PositionsPanel({ items, onChanged }: { items: Position[]; onChanged: () => void }) {
  const [editing, setEditing] = useState<Position | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function save(title: string, description: string) {
    try {
      if (editing === "new") {
        await apiFetch("/api/v1/positions", { method: "POST", body: JSON.stringify({ title, description }) });
      } else if (editing) {
        await apiFetch(`/api/v1/positions/${editing.id}`, { method: "PUT", body: JSON.stringify({ title, description }) });
      }
      setEditing(null);
      setError(null);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save position");
    }
  }

  async function toggleActive(id: number, next: boolean) {
    try {
      await apiFetch(`/api/v1/positions/${id}/active`, { method: "PATCH", body: JSON.stringify({ is_active: next }) });
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update position");
    }
  }

  return (
    <PanelShell addLabel="Add Position" onAddClick={() => setEditing("new")}>
      {error && <p className="mb-3 text-sm text-error">{error}</p>}
      <EntityList
        rows={items.map((p) => ({ id: p.id, primary: p.title, secondary: p.description, isActive: p.is_active }))}
        onEdit={(id) => setEditing(items.find((p) => p.id === id) ?? null)}
        onToggleActive={toggleActive}
      />
      {editing && (
        <NameDescriptionForm
          title={editing === "new" ? "Add Position" : "Edit Position"}
          nameLabel="Position title"
          initialName={editing === "new" ? "" : editing.title}
          initialDescription={editing === "new" ? "" : editing.description}
          onCancel={() => setEditing(null)}
          onSave={save}
        />
      )}
    </PanelShell>
  );
}

// Each department has exactly one designation (see backend: creating a
// second for the same department is rejected) — the department picker only
// ever offers departments that don't already have one, except the one this
// designation (if editing) already belongs to.
function DesignationsPanel({
  items,
  departments,
  onChanged,
}: {
  items: Designation[];
  departments: Department[];
  onChanged: () => void;
}) {
  const [editing, setEditing] = useState<Designation | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const takenDepartmentIds = new Set(items.map((d) => d.department_id));
  const availableDepartments = departments.filter(
    (d) => !takenDepartmentIds.has(d.id) || (editing !== "new" && editing?.department_id === d.id)
  );

  async function save(title: string, description: string, departmentId: number) {
    try {
      const body = JSON.stringify({ title, description, department_id: departmentId });
      if (editing === "new") {
        await apiFetch("/api/v1/designations", { method: "POST", body });
      } else if (editing) {
        await apiFetch(`/api/v1/designations/${editing.id}`, { method: "PUT", body });
      }
      setEditing(null);
      setError(null);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save designation");
    }
  }

  async function toggleActive(id: number, next: boolean) {
    try {
      await apiFetch(`/api/v1/designations/${id}/active`, { method: "PATCH", body: JSON.stringify({ is_active: next }) });
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update designation");
    }
  }

  return (
    <PanelShell addLabel="Add Designation" onAddClick={() => setEditing("new")}>
      {error && <p className="mb-3 text-sm text-error">{error}</p>}
      <EntityList
        rows={items.map((d) => ({
          id: d.id,
          primary: d.title,
          secondary: d.department_name,
          isActive: d.is_active,
        }))}
        onEdit={(id) => setEditing(items.find((d) => d.id === id) ?? null)}
        onToggleActive={toggleActive}
      />
      {editing && (
        <DesignationForm
          title={editing === "new" ? "Add Designation" : "Edit Designation"}
          initialTitle={editing === "new" ? "" : editing.title}
          initialDescription={editing === "new" ? "" : editing.description}
          initialDepartmentId={editing === "new" ? null : editing.department_id}
          availableDepartments={availableDepartments}
          onCancel={() => setEditing(null)}
          onSave={save}
        />
      )}
    </PanelShell>
  );
}

function NameDescriptionForm({
  title,
  nameLabel,
  initialName,
  initialDescription,
  onCancel,
  onSave,
}: {
  title: string;
  nameLabel: string;
  initialName: string;
  initialDescription: string;
  onCancel: () => void;
  onSave: (name: string, description: string) => void;
}) {
  const [name, setName] = useState(initialName);
  const [description, setDescription] = useState(initialDescription);
  const [saving, setSaving] = useState(false);

  return (
    <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p className="mb-3 text-sm font-semibold text-slate-800">{title}</p>
      <div className="mb-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">{nameLabel}</label>
        <input value={name} onChange={(e) => setName(e.target.value)} className="input" autoFocus />
      </div>
      <div className="mb-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">Description (optional)</label>
        <input value={description} onChange={(e) => setDescription(e.target.value)} className="input" />
      </div>
      <div className="flex gap-2">
        <button
          type="button"
          disabled={!name.trim() || saving}
          onClick={() => {
            setSaving(true);
            onSave(name.trim(), description.trim());
          }}
          className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
        >
          Save
        </button>
        <button type="button" onClick={onCancel} className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50">
          Cancel
        </button>
      </div>
    </div>
  );
}

function DesignationForm({
  title,
  initialTitle,
  initialDescription,
  initialDepartmentId,
  availableDepartments,
  onCancel,
  onSave,
}: {
  title: string;
  initialTitle: string;
  initialDescription: string;
  initialDepartmentId: number | null;
  availableDepartments: Department[];
  onCancel: () => void;
  onSave: (title: string, description: string, departmentId: number) => void;
}) {
  const [titleValue, setTitleValue] = useState(initialTitle);
  const [description, setDescription] = useState(initialDescription);
  const [departmentId, setDepartmentId] = useState<number | null>(initialDepartmentId);
  const [saving, setSaving] = useState(false);

  return (
    <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p className="mb-3 text-sm font-semibold text-slate-800">{title}</p>
      <div className="mb-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">Designation title</label>
        <input value={titleValue} onChange={(e) => setTitleValue(e.target.value)} className="input" autoFocus />
      </div>
      <div className="mb-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">Department</label>
        <select
          value={departmentId ?? ""}
          onChange={(e) => setDepartmentId(e.target.value ? Number(e.target.value) : null)}
          className="input"
        >
          <option value="">Select a department…</option>
          {availableDepartments.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>
        {availableDepartments.length === 0 && (
          <p className="mt-1 text-xs text-slate-400">Every department already has a designation.</p>
        )}
      </div>
      <div className="mb-3">
        <label className="mb-1 block text-xs font-semibold text-slate-600">Description (optional)</label>
        <input value={description} onChange={(e) => setDescription(e.target.value)} className="input" />
      </div>
      <div className="flex gap-2">
        <button
          type="button"
          disabled={!titleValue.trim() || !departmentId || saving}
          onClick={() => {
            setSaving(true);
            onSave(titleValue.trim(), description.trim(), departmentId!);
          }}
          className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue-dark disabled:cursor-not-allowed disabled:opacity-50"
        >
          Save
        </button>
        <button type="button" onClick={onCancel} className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50">
          Cancel
        </button>
      </div>
    </div>
  );
}
