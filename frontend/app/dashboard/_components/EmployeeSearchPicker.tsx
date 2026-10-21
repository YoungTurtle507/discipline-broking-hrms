"use client";

import { useState } from "react";
import { MagnifyingGlass } from "@phosphor-icons/react";
import { Avatar } from "@/components/Avatar";
import type { Employee } from "../employees/_components/types";

const MAX_RESULTS = 8;

// A ranked, type-to-search employee picker — the same interaction the
// attendance correction modal originally had on its own, now shared with
// every other spot that needs to find one specific employee out of the full
// roster (the "Today's Attendance" filter, "Check balance for") instead of
// each re-implementing the ranking or falling back to a plain scrolling
// <select>. Nothing shows until there's something to search for; once
// typing, a name or code that STARTS WITH the query outranks one that
// merely contains it somewhere in the middle, matching what someone expects
// while typing a name, and results are capped so the dropdown never grows
// unreasonably long.
export function EmployeeSearchPicker({
  employees,
  selected,
  onSelect,
  placeholder = "Search by name or employee code…",
  autoFocus = false,
  changeLabel = "Change",
}: {
  employees: Employee[];
  selected: Employee | null;
  // null clears the selection — a filter's "show everyone again", or a
  // picker's "nothing chosen yet", depending on the caller.
  onSelect: (employee: Employee | null) => void;
  placeholder?: string;
  autoFocus?: boolean;
  changeLabel?: string;
}) {
  const [search, setSearch] = useState("");
  const [highlighted, setHighlighted] = useState(0);

  const needle = search.trim().toLowerCase();
  const filtered =
    needle === ""
      ? []
      : employees
          .map((e) => {
            const name = e.full_name.toLowerCase();
            const code = e.employee_code.toLowerCase();
            let rank = -1;
            if (name.startsWith(needle) || code.startsWith(needle)) rank = 0;
            else if (name.includes(needle) || code.includes(needle)) rank = 1;
            return { e, rank };
          })
          .filter((r) => r.rank >= 0)
          .sort((a, b) => a.rank - b.rank || a.e.full_name.localeCompare(b.e.full_name))
          .map((r) => r.e)
          .slice(0, MAX_RESULTS);

  function handleSearchChange(value: string) {
    setSearch(value);
    // Pins the keyboard highlight back to the top row whenever the ranked
    // list underneath changes, rather than leaving it pointing at whatever
    // index happened to be highlighted in the previous list.
    setHighlighted(0);
  }

  function handleSearchKeyDown(ev: React.KeyboardEvent<HTMLInputElement>) {
    if (filtered.length === 0) return;
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      setHighlighted((i) => Math.min(i + 1, filtered.length - 1));
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      setHighlighted((i) => Math.max(i - 1, 0));
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      const pick = filtered[highlighted];
      if (pick) {
        onSelect(pick);
        setSearch("");
      }
    }
  }

  if (selected) {
    return (
      <div className="flex items-center gap-3 rounded-lg border border-slate-200 px-3 py-2.5">
        <Avatar name={selected.full_name} employeeId={selected.id} size={32} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-slate-800">{selected.full_name}</p>
          <p className="text-xs text-slate-400">{selected.employee_code}</p>
        </div>
        <button
          type="button"
          onClick={() => onSelect(null)}
          className="shrink-0 text-xs font-semibold text-brand-blue hover:underline"
        >
          {changeLabel}
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="relative">
        <MagnifyingGlass size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
        <input
          value={search}
          onChange={(ev) => handleSearchChange(ev.target.value)}
          onKeyDown={handleSearchKeyDown}
          placeholder={placeholder}
          className="input pl-9"
          autoFocus={autoFocus}
        />
      </div>
      {filtered.length > 0 ? (
        <div className="mt-2 max-h-52 divide-y divide-slate-100 overflow-y-auto rounded-lg border border-slate-200">
          {filtered.map((emp, i) => (
            <button
              key={emp.id}
              type="button"
              onClick={() => {
                onSelect(emp);
                setSearch("");
              }}
              onMouseEnter={() => setHighlighted(i)}
              className={`flex w-full items-center gap-3 px-3 py-2 text-left ${
                i === highlighted ? "bg-light-blue" : "hover:bg-slate-50"
              }`}
            >
              <Avatar name={emp.full_name} employeeId={emp.id} size={30} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-slate-800">{emp.full_name}</span>
                <span className="block truncate text-xs text-slate-400">
                  {emp.employee_code}
                  {emp.department_name ? ` · ${emp.department_name}` : ""}
                </span>
              </span>
            </button>
          ))}
        </div>
      ) : needle !== "" ? (
        <p className="mt-2 text-xs text-slate-400">No employees match &ldquo;{search}&rdquo;.</p>
      ) : null}
    </div>
  );
}
