"use client";

import Link from "next/link";
import { ScanSmiley, UserPlus } from "@phosphor-icons/react/ssr";
import { useSession } from "./SessionProvider";

const ACTIONS = [
  {
    icon: ScanSmiley,
    label: "Mark Attendance",
    href: "/dashboard/attendance?auto=1",
  },
  {
    icon: UserPlus,
    label: "Add Employee",
    href: "/dashboard/employees/new",
    // Only worth offering as a shortcut to a login that can actually use it —
    // the Employees page itself already hides the same button for a
    // view-only (employees.view-only) login.
    permission: "employees.manage",
  },
];

// A slim, scannable action bar rather than a card of its own.
export function QuickActions() {
  const { can } = useSession();
  const visibleActions = ACTIONS.filter((a) => !a.permission || can(a.permission));

  return (
    <div className="flex flex-wrap items-center gap-3 border border-slate-200 bg-white px-4 py-2.5">
      <span className="font-mono text-[10px] font-semibold uppercase tracking-widest text-slate-400">
        Frequent Actions
      </span>
      <div className="flex flex-1 flex-wrap gap-2">
        {visibleActions.map(({ icon: Icon, label, href }) => (
          <Link
            key={label}
            href={href}
            className="flex items-center gap-1.5 border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:border-brand-blue hover:bg-slate-50"
          >
            <Icon size={14} weight="bold" className="text-brand-blue" />
            {label}
          </Link>
        ))}
      </div>
    </div>
  );
}
