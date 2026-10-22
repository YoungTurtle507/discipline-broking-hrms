"use client";

import { useEffect, useState } from "react";
import { Sidebar } from "./Sidebar";
import { SessionProvider } from "./SessionProvider";
import { Topbar } from "./Topbar";
import { useDailyReload } from "./useDailyReload";

const SIDEBAR_COLLAPSED_KEY = "hrms-sidebar-collapsed";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  // Desktop-only "hide the sidebar to get more screen width back" toggle —
  // remembered per browser via localStorage so it stays hidden across page
  // navigations and reloads until deliberately unhidden again. Starts false
  // on the server/first paint (no window there) and syncs from storage right
  // after mount; a returning user with it collapsed sees one brief flash of
  // the full sidebar before it re-collapses, which is preferable to an
  // SSR/client markup mismatch.
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  useEffect(() => {
    try {
      setSidebarCollapsed(localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1");
    } catch {
      // Private-window/blocked storage — just keep the default (expanded).
    }
  }, []);

  function toggleSidebarCollapsed() {
    setSidebarCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        // Nothing to persist to — the toggle still works for this page view.
      }
      return next;
    });
  }

  // Refreshes every dashboard session once a day at 8:30 AM, so a screen left
  // open overnight always shows the new day's attendance instead of stale
  // data from before midnight.
  useDailyReload(8, 30);

  return (
    <SessionProvider>
      <div className="flex h-screen overflow-hidden bg-slate-50">
        <Sidebar
          open={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
          collapsed={sidebarCollapsed}
          onToggleCollapse={toggleSidebarCollapsed}
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <Topbar onMenuClick={() => setSidebarOpen(true)} />
          {/* Capped past ~1600px so content stays proportioned on very large
              / ultra-wide monitors instead of stretching edge to edge —
              below the cap (everything through desktop/laptop widths) this
              has no effect at all. */}
          <main className="@container flex-1 overflow-y-auto p-4 sm:p-5 lg:p-6">
            <div className="mx-auto max-w-[1600px]">{children}</div>
          </main>
        </div>
      </div>
    </SessionProvider>
  );
}
