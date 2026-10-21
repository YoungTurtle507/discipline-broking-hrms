"use client";

import { useEffect, useState } from "react";
import { Clock } from "@phosphor-icons/react";
import { apiFetch } from "@/lib/api";

// Shows the software's own clock — the same one every attendance calculation
// treats as "now" (see backend/internal/clock), not the viewer's unadjusted
// system clock, which may run a little ahead of it. Synced from the server
// once, then ticked forward locally by a plain 1-second timer (so this isn't
// re-fetching every second) and re-synced periodically to correct for any
// drift between the two clocks.
const RESYNC_MS = 5 * 60_000;

export function DigitalClock() {
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    let cancelled = false;
    let offsetMs = 0;
    let tickTimer: ReturnType<typeof setInterval> | undefined;

    async function sync() {
      try {
        const res = await apiFetch<{ time: string }>("/api/v1/clock");
        offsetMs = new Date(res.time).getTime() - Date.now();
      } catch {
        // Keep whatever offset we already have; the next resync will correct it.
      }
    }

    sync().then(() => {
      if (cancelled) return;
      setNow(new Date(Date.now() + offsetMs));
      tickTimer = setInterval(() => {
        if (!cancelled) setNow(new Date(Date.now() + offsetMs));
      }, 1000);
    });

    const resyncTimer = setInterval(sync, RESYNC_MS);

    return () => {
      cancelled = true;
      clearInterval(resyncTimer);
      if (tickTimer) clearInterval(tickTimer);
    };
  }, []);

  return (
    <span className="flex items-center gap-1.5 font-mono text-xs font-semibold tabular-nums text-slate-700">
      <Clock size={14} className="text-slate-400" />
      {now
        ? now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
        : "--:--:--"}
    </span>
  );
}
