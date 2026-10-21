"use client";

import { useState } from "react";
import { getBackendUrl } from "@/lib/api";

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/);
  const first = parts[0]?.[0] ?? "";
  const last = parts.length > 1 ? parts[parts.length - 1][0] : "";
  return (first + last).toUpperCase();
}

// Ops Terminal drops per-person color-coding — a dense utility surface reads
// as one accent doing the work, not a palette of identity colors. A plain
// bordered square with mono initials fits the same "no icons, hairlines only"
// rule the rest of the shell follows.
//
// When employeeId is given, this tries that employee's own registered face
// photo (the same FRONT capture used for face-scan attendance) first, and
// only falls back to the initials square if they don't have one yet (or it
// fails to load) — never a placeholder image, never a separate upload.
export function Avatar({
  name,
  employeeId,
  size = 40,
}: {
  name: string;
  employeeId?: number;
  size?: number;
}) {
  const [photoFailed, setPhotoFailed] = useState(false);

  if (employeeId && !photoFailed) {
    return (
      <img
        src={`${getBackendUrl()}/api/v1/employees/${employeeId}/face-photo`}
        alt={name}
        onError={() => setPhotoFailed(true)}
        className="inline-block shrink-0 border border-slate-300 bg-slate-50 object-cover"
        style={{ width: size, height: size }}
      />
    );
  }

  return (
    <span
      className="inline-flex shrink-0 items-center justify-center border border-slate-300 bg-slate-50 font-mono font-semibold text-slate-700"
      style={{
        width: size,
        height: size,
        fontSize: size * 0.34,
      }}
      aria-hidden
    >
      {getInitials(name)}
    </span>
  );
}
