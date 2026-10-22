"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { X, SidebarSimple, SquaresFour, Users, Fingerprint, Buildings, ChartBar, type Icon } from "@phosphor-icons/react";
import { useSession } from "./SessionProvider";

// `permission` is the backend permission code that gates the page's API. An
// item with none is open to anyone signed in. `employeeOnly` items are the
// personal views, which only make sense for a login linked to an employee
// record — an HR or admin account has no attendance of its own. `requireRole`
// checks the literal role name instead — unlike `permission` (via `can()`),
// it does NOT let Super Admin's usual "sees everything" rule through, for a
// module that's deliberately Admin-only, full stop (see the backend's
// matching auth.RequireRole on the Candidates API).
type NavItem = {
  label: string;
  href: string;
  icon: Icon;
  enabled: boolean;
  permission?: string;
  requireRole?: string;
  employeeOnly?: boolean;
};

// Only two logins exist in this duplicate: Super Admin (bypasses every
// permission check) and the shared kiosk account (employees.view +
// attendance.manage — see backend/migrations/000050_add_kiosk_role).
// Organization is Super Admin only, full stop (requireRole, not permission)
// — the kiosk login has no business renaming departments.
const NAV_ITEMS: NavItem[] = [
  { label: "Dashboard", href: "/dashboard", icon: SquaresFour, enabled: true },
  { label: "Employees", href: "/dashboard/employees", icon: Users, enabled: true, permission: "employees.view" },
  { label: "Attendance", href: "/dashboard/attendance", icon: Fingerprint, enabled: true, permission: "attendance.manage" },
  { label: "Organization", href: "/dashboard/organization", icon: Buildings, enabled: true, requireRole: "SUPER_ADMIN" },
  { label: "Reports", href: "/dashboard/reports", icon: ChartBar, enabled: true, requireRole: "SUPER_ADMIN" },
];

// A shared spring for every size/position change in this file — one motion
// "personality" (snappy, no overshoot) rather than each element picking its
// own feel.
const SPRING = { type: "spring" as const, stiffness: 420, damping: 34, mass: 0.9 };


export function Sidebar({
  open,
  onClose,
  collapsed,
  onToggleCollapse,
}: {
  open: boolean;
  onClose: () => void;
  // Desktop-only — mobile always uses the full `open` overlay drawer
  // regardless of this, since it's already off-canvas by default there.
  collapsed: boolean;
  // Flips `collapsed` either direction — called from the button inside the
  // full sidebar (collapse it) and from the one inside the narrow rail
  // (pin it back open), so both live right where you're already looking
  // instead of forcing a trip back to the Topbar's own toggle.
  onToggleCollapse: () => void;
}) {
  const pathname = usePathname();
  const { can, me, employee, loading } = useSession();
  const reduceMotion = useReducedMotion();
  // Drives the rail's hover-elaborate — framer-motion's own hover handlers
  // rather than raw CSS :hover, so the width itself is a spring the same way
  // every other move in this file is, not a linear CSS transition.
  const [railHovered, setRailHovered] = useState(false);

  // Showing a link the user will only get a 403 from is worse than showing
  // nothing, so hide until the session is known, then filter.
  const visibleItems = loading
    ? []
    : NAV_ITEMS.filter((item) => {
        if (item.employeeOnly) return employee !== null;
        if (item.requireRole) return me?.roles.includes(item.requireRole) ?? false;
        if (item.permission) return can(item.permission);
        return true;
      });

  return (
    <>
      {open && (
        <motion.div
          className="fixed inset-0 z-40 bg-slate-900/40 lg:hidden"
          onClick={onClose}
          aria-hidden
          initial={reduceMotion ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        />
      )}

      <aside
        className={[
          "fixed inset-y-0 left-0 z-50 flex h-full w-56 shrink-0 -translate-x-full flex-col border-r border-slate-200 bg-white text-slate-600 transition-transform duration-150 ease-out",
          collapsed ? "lg:relative lg:w-16" : "lg:w-56",
          "lg:static lg:translate-x-0",
          open ? "translate-x-0" : "",
        ].join(" ")}
      >
        {/* Full content — always shown on mobile (collapsing is desktop-only),
            and on desktop whenever the sidebar isn't collapsed. */}
        <div className={collapsed ? "flex h-full flex-col px-4 py-4.5 lg:hidden" : "flex h-full flex-col px-4 py-4.5"}>
          {/* The logo is a light/white mark meant for a dark ground — this band
              gives it one, using the app's own brand-blue rather than an
              arbitrary color, while the rest of the sidebar stays light. Fixed
              at h-16 to match the Topbar's own height exactly (py-2.5 around a
              36px avatar button there), so the two line up edge to edge. */}
          <div className="-mx-4 -mt-4.5 mb-5 flex h-16 items-center justify-between bg-navy px-4">
            {/* Text wordmark until the firm's logo is ready — swap for an
                <Image>, same as the other two spots below, once one exists. */}
            <span className="truncate text-[17px] font-extrabold tracking-tight text-white">Discipline Broking</span>
            <div className="flex items-center gap-1">
              {/* The one and only sidebar hide/show control — top-right of
                  the sidebar's own header, nowhere else (not the Topbar, not
                  a second button lower down). Desktop only; mobile's own
                  close (below) covers the same job there. */}
              <motion.button
                type="button"
                onClick={onToggleCollapse}
                whileHover={{ backgroundColor: "rgba(255,255,255,0.12)" }}
                whileTap={{ scale: 0.88 }}
                className="hidden h-7 w-7 shrink-0 items-center justify-center text-white/70 hover:text-white lg:flex"
                title="Hide sidebar"
                aria-label="Hide sidebar"
              >
                <SidebarSimple size={16} />
              </motion.button>
              <button
                type="button"
                onClick={onClose}
                className="flex h-7 w-7 shrink-0 items-center justify-center text-white/70 hover:bg-white/10 hover:text-white lg:hidden"
                aria-label="Close menu"
              >
                <X size={16} />
              </button>
            </div>
          </div>
          <p className="mb-5 px-1 font-mono text-[10px] tracking-wider text-slate-400">HRMS / OPS-16.2</p>

          <p className="mb-1.5 px-1 font-mono text-[9.5px] font-semibold uppercase tracking-widest text-slate-400">
            Modules
          </p>
          <nav className="flex flex-1 flex-col">
            {visibleItems.map(({ label, href, icon: ItemIcon, enabled }) => {
              const active = enabled && pathname === href;
              const rowClassName = [
                "relative -ml-px flex items-center gap-2.5 px-3 py-[5px] text-[13px] transition-colors",
                enabled ? (active ? "font-semibold text-slate-900" : "text-slate-600 hover:text-slate-900") : "cursor-not-allowed text-slate-300",
              ].join(" ");

              // One shared element, matched by layoutId, that glides between
              // whichever row is active instead of each row drawing its own
              // static highlight — the same technique behind Linear/Vercel's
              // "sliding tab" feel. It even glides smoothly into the
              // collapsed rail's own version of this indicator (below) on
              // the very same layoutId, since only one is ever mounted at a
              // time.
              const indicator = active && (
                <motion.span
                  layoutId="sidebar-active-indicator"
                  transition={reduceMotion ? { duration: 0 } : SPRING}
                  className="absolute inset-0 border-l-2 border-brand-blue bg-light-blue"
                />
              );

              const content = (
                <>
                  <ItemIcon size={16} weight={active ? "fill" : "regular"} className="relative z-10 shrink-0" />
                  <span className="relative z-10">{label}</span>
                </>
              );

              if (!enabled) {
                return (
                  <span key={label} className={rowClassName} title="Coming soon" aria-disabled>
                    {indicator}
                    {content}
                  </span>
                );
              }

              return (
                <Link key={label} href={href} className={rowClassName}>
                  {indicator}
                  {content}
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Collapsed rail — desktop only. Reserves a plain 64px column (never
            reflows the page); hovering it "elaborates" the same element from
            64px to 224px on a spring, revealing the labels that were there
            all along, then retracts the moment the pointer leaves.
            Absolutely positioned so growing it never pushes the page's main
            content around — the 64px the aside itself reserves is what
            actually keeps that content offset. The header icon — the same
            toggle as the expanded sidebar's own, nothing new — is what
            fixes the sidebar open again; no second control at the bottom of
            the rail. */}
        {collapsed && (
          <motion.div
            onHoverStart={() => setRailHovered(true)}
            onHoverEnd={() => setRailHovered(false)}
            initial={false}
            animate={{ width: railHovered ? 224 : 64 }}
            transition={reduceMotion ? { duration: 0 } : SPRING}
            className="absolute inset-y-0 left-0 z-40 hidden flex-col overflow-hidden border-r border-slate-200 bg-white shadow-[0_12px_32px_-8px_rgba(22,23,26,0.18)] lg:flex"
          >
            {/* Header crossfades between two looks: at rest, just the
                compact "DB" mark (text until the firm's logo is ready) —
                hovered ("elaborated"), the exact same header the fixed
                sidebar itself shows: full wordmark plus the pin-open icon —
                so hovering genuinely previews what clicking to fix it open
                gives you. */}
            <div className="relative h-16 w-56 shrink-0 overflow-hidden border-b border-slate-100 bg-navy">
              <AnimatePresence initial={false}>
                {railHovered ? (
                  <motion.div
                    key="full"
                    initial={reduceMotion ? false : { opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.12 }}
                    className="absolute inset-0 flex items-center justify-between px-4"
                  >
                    <span className="truncate text-[17px] font-extrabold tracking-tight text-white">Discipline Broking</span>
                    <motion.button
                      type="button"
                      onClick={onToggleCollapse}
                      whileHover={{ backgroundColor: "rgba(255,255,255,0.12)" }}
                      whileTap={{ scale: 0.88 }}
                      title="Fix sidebar open"
                      aria-label="Fix sidebar open"
                      className="flex h-7 w-7 shrink-0 items-center justify-center text-white/70 hover:text-white"
                    >
                      <SidebarSimple size={16} />
                    </motion.button>
                  </motion.div>
                ) : (
                  <motion.button
                    key="mark"
                    type="button"
                    onClick={onToggleCollapse}
                    initial={reduceMotion ? false : { opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.12 }}
                    title="Show full sidebar"
                    aria-label="Show full sidebar"
                    className="absolute inset-0 flex items-center"
                  >
                    {/* Centered within the rail's own 64px resting width, not
                        this header's full 224px — the header itself stays
                        224px at all times (see the rest of this file's
                        w-56 rows), only revealed a bit at a time as the
                        rail's own width animates, so centering against the
                        header directly would center it out past the visible
                        edge. */}
                    <span className="flex h-16 w-16 shrink-0 items-center justify-center text-base font-extrabold tracking-tight text-white">
                      DB
                    </span>
                  </motion.button>
                )}
              </AnimatePresence>
            </div>
            {/* Same "Modules" section label the fixed sidebar shows above
                its own nav — fades in with the rest of the reveal, stays
                invisible at the 64px resting width like everything else
                here that isn't the icons themselves. */}
            <motion.p
              initial={false}
              animate={{ opacity: railHovered ? 1 : 0 }}
              transition={reduceMotion ? { duration: 0 } : { duration: 0.16 }}
              className="w-56 whitespace-nowrap px-1 pb-1.5 pl-4 pt-3 font-mono text-[9.5px] font-semibold uppercase tracking-widest text-slate-400"
            >
              Modules
            </motion.p>
            <nav className="flex w-56 flex-1 flex-col gap-1">
              {visibleItems.map(({ label, href, icon: ItemIcon, enabled }, i) => {
                const active = enabled && pathname === href;
                // Same type size, weight rule, and vertical rhythm as the
                // fixed sidebar's own rows (py-[5px], text-[13px], font-
                // semibold only when active) — hovering this rail is
                // supposed to preview the fixed sidebar exactly, so a row
                // here must never look larger or bolder than its fixed
                // counterpart. Left padding is 4px more than the fixed
                // rows' (16px vs 12px) purely so the label stays fully
                // clipped behind the icon at the 64px resting width —
                // everything else matches.
                const rowClassName = [
                  "relative flex w-56 items-center gap-2.5 whitespace-nowrap py-[5px] pl-4 pr-3 text-[13px] transition-colors",
                  enabled
                    ? active
                      ? "font-semibold text-slate-900"
                      : "text-slate-600 hover:text-slate-900"
                    : "cursor-not-allowed text-slate-300",
                ].join(" ");

                const indicator = active && (
                  <motion.span
                    layoutId="sidebar-active-indicator"
                    transition={reduceMotion ? { duration: 0 } : SPRING}
                    className="absolute inset-0 border-l-2 border-brand-blue bg-light-blue"
                  />
                );

                // Staggered by row index so the labels cascade in on open
                // rather than popping in all at once — snaps straight to
                // hidden on close, no stagger, so hiding still feels instant.
                const labelMotion = {
                  initial: false as const,
                  animate: { opacity: railHovered ? 1 : 0 },
                  transition: reduceMotion
                    ? { duration: 0 }
                    : { duration: 0.16, delay: railHovered ? i * 0.018 : 0 },
                };

                const body = (
                  <>
                    <ItemIcon size={16} weight={active ? "fill" : "regular"} className="relative z-10 shrink-0" />
                    <motion.span className="relative z-10" {...labelMotion}>
                      {label}
                    </motion.span>
                  </>
                );

                if (!enabled) {
                  return (
                    <span key={label} className={rowClassName} title="Coming soon" aria-disabled>
                      {indicator}
                      {body}
                    </span>
                  );
                }

                return (
                  <Link key={label} href={href} className={rowClassName} aria-label={label}>
                    {indicator}
                    {body}
                  </Link>
                );
              })}
            </nav>
          </motion.div>
        )}
      </aside>
    </>
  );
}
