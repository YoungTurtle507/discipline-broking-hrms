"use client";

import { useEffect } from "react";

// Schedules a full page reload once a day at the given local time, so a
// browser tab left open overnight (e.g. an always-on office display) picks
// up the new day's data automatically instead of showing yesterday's numbers
// until someone manually refreshes. A reload doesn't touch the session — it
// lives in an httpOnly cookie the browser resends automatically, and the
// reload itself only extends the session's sliding expiry, same as any other
// request would.
export function useDailyReload(hour: number, minute: number) {
  useEffect(() => {
    const loadedOn = new Date().toDateString();

    function msUntilNext(): number {
      const now = new Date();
      const next = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hour, minute, 0, 0);
      if (next <= now) next.setDate(next.getDate() + 1);
      return next.getTime() - now.getTime();
    }

    const timer = setTimeout(() => window.location.reload(), msUntilNext());

    // Safety net for a machine that was asleep or a background tab whose
    // timers got throttled through the scheduled time: catch up as soon as
    // the tab is looked at again if the calendar day has actually moved on
    // since this page was loaded, rather than waiting on the exact minute.
    function handleVisibility() {
      if (document.visibilityState === "visible" && new Date().toDateString() !== loadedOn) {
        window.location.reload();
      }
    }
    document.addEventListener("visibilitychange", handleVisibility);

    return () => {
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, [hour, minute]);
}
