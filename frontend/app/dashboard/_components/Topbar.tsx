"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { List, MagnifyingGlass, CaretDown, SignOut, Sparkle } from "@phosphor-icons/react";
import { Avatar } from "@/components/Avatar";
import { apiFetch } from "@/lib/api";
import { useSession } from "./SessionProvider";
import { EmployeeStatusBadge } from "../employees/_components/EmployeeStatusBadge";
import type { Employee } from "../employees/_components/types";
import { parseSmartQuery, type SmartQuery } from "./smartSearchQuery";
import { formatDate } from "./my/types";

const MAX_RESULTS = 6;

type SmartAnswerState =
  | { status: "loading"; query: SmartQuery }
  | { status: "ready"; query: SmartQuery; value: number }
  | { status: "empty"; query: SmartQuery }
  | { status: "error"; query: SmartQuery; message: string };

function matches(e: Employee, needle: string): boolean {
  const haystack = [e.full_name, e.employee_code, e.department_name, e.designation_title, e.status]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  return haystack.includes(needle);
}

function formatRole(roles: string[]): string {
  if (roles.length === 0) return "";
  return roles[0]
    .split("_")
    .map((w) => w[0] + w.slice(1).toLowerCase())
    .join(" ");
}

export function Topbar({ onMenuClick }: { onMenuClick: () => void }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const { me, employee, can } = useSession();
  const canSearch = can("employees.manage");
  const [menuOpen, setMenuOpen] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [menuOpen]);

  useEffect(() => {
    if (!canSearch) return;
    apiFetch<{ employees: Employee[] | null }>("/api/v1/employees")
      .then((d) => setEmployees(d.employees ?? []))
      .catch(() => setEmployees([]));
  }, [canSearch]);

  useEffect(() => {
    if (!searchOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setSearchOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [searchOpen]);

  const needle = query.trim().toLowerCase();
  const results = useMemo(() => {
    if (!needle) return [];
    return employees.filter((e) => matches(e, needle));
  }, [employees, needle]);

  // "how many days did chetan come late this month" — a fixed set of
  // attendance questions answered straight from the search bar instead of
  // sending HR through Reports for something this specific. Debounced so a
  // resolved question doesn't refetch on every further keystroke.
  const smartQuery = useMemo(() => (canSearch ? parseSmartQuery(query, employees) : null), [canSearch, query, employees]);
  const [smartAnswer, setSmartAnswer] = useState<SmartAnswerState | null>(null);

  useEffect(() => {
    if (!smartQuery) return;
    const timer = setTimeout(() => {
      setSmartAnswer({ status: "loading", query: smartQuery });
      apiFetch<{ employees: Array<Record<string, unknown>> }>(
        `/api/v1/reports/attendance-summary?employee_id=${smartQuery.employee.id}&from=${smartQuery.from}&to=${smartQuery.to}`
      )
        .then((d) => {
          const row = d.employees?.[0];
          if (!row || !(smartQuery.field in row)) {
            setSmartAnswer({ status: "empty", query: smartQuery });
            return;
          }
          setSmartAnswer({ status: "ready", query: smartQuery, value: Number(row[smartQuery.field]) });
        })
        .catch(() => setSmartAnswer({ status: "error", query: smartQuery, message: "Couldn't load that just now." }));
    }, 350);
    return () => clearTimeout(timer);
  }, [smartQuery]);

  function goToEmployee(id: number) {
    router.push(`/dashboard/employees/${id}`);
    setQuery("");
    setSearchOpen(false);
  }

  function handleSearchKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" && needle) {
      router.push(`/dashboard/employees?q=${encodeURIComponent(query.trim())}`);
      setSearchOpen(false);
    } else if (e.key === "Escape") {
      setSearchOpen(false);
    }
  }

  async function handleLogout() {
    setLoggingOut(true);
    try {
      await apiFetch("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // Even if the request fails, the cookie may already be gone or the
      // session already expired — send them to login regardless.
    } finally {
      router.push("/login");
    }
  }

  const displayName = me?.full_name ?? "";
  const displayRole = me ? formatRole(me.roles) : "";

  return (
    <header className="flex h-16 items-center gap-3 border-b border-slate-200 bg-white px-4 py-2.5 sm:gap-4 sm:px-6 lg:px-8">
      <button
        type="button"
        onClick={onMenuClick}
        className="flex h-9 w-9 shrink-0 items-center justify-center text-slate-500 hover:bg-slate-100 lg:hidden"
        aria-label="Open menu"
      >
        <List size={20} />
      </button>

      <div className="relative min-w-0 max-w-md flex-1" ref={searchRef}>
        <MagnifyingGlass
          size={15}
          className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
        />
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setSearchOpen(true);
            // Clear any stale answer from the previous query right away —
            // the effect below will repopulate it if the new text still
            // resolves to a smart question, once its debounce settles.
            setSmartAnswer(null);
          }}
          onFocus={() => setSearchOpen(true)}
          onKeyDown={handleSearchKeyDown}
          type="text"
          disabled={!canSearch}
          suppressHydrationWarning
          placeholder={canSearch ? "name, department… or ask e.g. “late days this month”" : "Search unavailable for your role"}
          className="w-full border border-slate-300 bg-white py-1.5 pl-8 pr-3 font-mono text-xs text-slate-700 outline-none transition-colors placeholder:text-slate-400 focus:border-brand-blue disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400"
        />

        {searchOpen && needle && (
          <div className="absolute left-0 right-0 top-full z-20 mt-1.5 max-h-80 overflow-y-auto rounded-lg border border-slate-200 bg-white shadow-card-hover">
            {smartAnswer && (
              <div className="border-b border-slate-100 bg-light-blue/40 px-3.5 py-3">
                <div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wide text-brand-blue">
                  <Sparkle size={12} weight="fill" />
                  Smart answer
                </div>
                {smartAnswer.status === "loading" && <p className="text-xs text-slate-500">Checking…</p>}
                {smartAnswer.status === "error" && <p className="text-xs text-error">{smartAnswer.message}</p>}
                {smartAnswer.status === "empty" && (
                  <p className="text-xs text-slate-500">
                    No attendance data for {smartAnswer.query.employee.full_name} in {smartAnswer.query.rangeLabel}.
                  </p>
                )}
                {smartAnswer.status === "ready" && (
                  <>
                    <p className="text-[13px] text-slate-800">
                      <button
                        type="button"
                        onClick={() => goToEmployee(smartAnswer.query.employee.id)}
                        className="font-semibold text-brand-blue hover:underline"
                      >
                        {smartAnswer.query.employee.full_name}
                      </button>{" "}
                      {smartAnswer.query.describe(smartAnswer.value)} in {smartAnswer.query.rangeLabel}.
                    </p>
                    <p className="mt-1 text-[11px] text-slate-400">
                      {formatDate(smartAnswer.query.from)} – {formatDate(smartAnswer.query.to)} ·{" "}
                      <button
                        type="button"
                        onClick={() => {
                          router.push(
                            `/dashboard/reports/employee/${smartAnswer.query.employee.id}?from=${smartAnswer.query.from}&to=${smartAnswer.query.to}`
                          );
                          setSearchOpen(false);
                        }}
                        className="font-semibold text-brand-blue hover:underline"
                      >
                        View day-by-day →
                      </button>
                    </p>
                  </>
                )}
              </div>
            )}
            {results.length === 0 ? (
              smartAnswer ? null : (
                <p className="px-3.5 py-3 text-xs text-slate-400">No employees match &ldquo;{query.trim()}&rdquo;.</p>
              )
            ) : (
              <>
                <ul>
                  {results.slice(0, MAX_RESULTS).map((e) => (
                    <li key={e.id}>
                      <button
                        type="button"
                        onClick={() => goToEmployee(e.id)}
                        className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left hover:bg-slate-50"
                      >
                        <Avatar name={e.full_name} employeeId={e.id} size={28} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-[13px] font-semibold text-slate-800">{e.full_name}</span>
                          <span className="block truncate text-[11px] text-slate-400">
                            {e.employee_code}
                            {e.department_name ? ` · ${e.department_name}` : ""}
                            {e.designation_title ? ` · ${e.designation_title}` : ""}
                          </span>
                        </span>
                        <EmployeeStatusBadge status={e.status} />
                      </button>
                    </li>
                  ))}
                </ul>
                {results.length > MAX_RESULTS && (
                  <button
                    type="button"
                    onClick={() => {
                      router.push(`/dashboard/employees?q=${encodeURIComponent(query.trim())}`);
                      setSearchOpen(false);
                    }}
                    className="block w-full border-t border-slate-100 px-3.5 py-2 text-left text-xs font-semibold text-brand-blue hover:bg-slate-50"
                  >
                    View all {results.length} results in Employees →
                  </button>
                )}
              </>
            )}
          </div>
        )}
      </div>

      <div className="ml-auto flex items-center gap-4">
        <div className="relative" ref={menuRef}>
          <button
            type="button"
            onClick={() => setMenuOpen((v) => !v)}
            aria-expanded={menuOpen}
            className="flex items-center gap-2.5 py-1 pl-1 pr-2 transition-colors hover:bg-slate-100"
          >
            <Avatar name={displayName} employeeId={employee?.employee_id} size={36} />
            <span className="hidden text-left leading-tight sm:inline-block">
              <span className="block text-sm font-semibold text-slate-800">{displayName}</span>
              <span className="block text-xs text-slate-400">{displayRole}</span>
            </span>
            <CaretDown size={14} className={`hidden text-slate-400 transition-transform sm:block ${menuOpen ? "rotate-180" : ""}`} />
          </button>

          {menuOpen && (
            <div className="absolute right-0 top-full z-20 mt-2 w-48 overflow-hidden rounded-lg border border-slate-200 bg-white py-1.5 shadow-card-hover">
              <div className="border-b border-slate-100 px-3.5 py-2.5 sm:hidden">
                <p className="truncate text-sm font-semibold text-slate-800">{displayName}</p>
                <p className="truncate text-xs text-slate-400">{displayRole}</p>
              </div>
              <button
                type="button"
                disabled={loggingOut}
                onClick={handleLogout}
                className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left text-sm font-medium text-error transition-colors hover:bg-error-bg disabled:opacity-60"
              >
                <SignOut size={16} weight="bold" />
                {loggingOut ? "Logging out…" : "Logout"}
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
