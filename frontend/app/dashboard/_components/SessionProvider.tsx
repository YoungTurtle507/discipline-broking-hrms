"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";

export type Me = {
  user_id: number;
  username: string;
  full_name: string;
  roles: string[];
  permissions: string[];
};

// The signed-in user's own employee record, when their login is linked to one.
// Staff logins (HR/admin created in Settings) have no employee row, so this
// stays null and the "My…" parts of the app stay hidden for them.
export type MyProfile = {
  employee_id: number;
  employee_code: string;
  full_name: string;
  email: string;
  phone: string;
  joining_date: string;
  department_name: string | null;
  designation_title: string | null;
  shift_name: string | null;
  shift_start: string | null;
  shift_end: string | null;
  grace_period_minutes: number | null;
  reporting_manager_name: string | null;
  status: string;
  last_working_day: string | null;
  // Whether this employee may mark their own attendance from their dashboard
  // with no face verification — set per-employee by an admin, for someone
  // with no camera to scan against (e.g. working from home).
  remote_self_checkin: boolean;
};

type Session = {
  me: Me | null;
  employee: MyProfile | null;
  loading: boolean;
  can: (permission: string) => boolean;
};

const SessionContext = createContext<Session>({
  me: null,
  employee: null,
  loading: true,
  can: () => false,
});

export function useSession() {
  return useContext(SessionContext);
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [employee, setEmployee] = useState<MyProfile | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const identity = await apiFetch<Me>("/api/v1/auth/me");
        if (cancelled) return;
        setMe(identity);

        // Only worth asking when the account could have one — a 404 here is a
        // normal answer for a staff login, not a failure.
        if (identity.permissions.includes("profile.view_own")) {
          try {
            const profile = await apiFetch<MyProfile>("/api/v1/me/profile");
            if (!cancelled) setEmployee(profile);
          } catch {
            if (!cancelled) setEmployee(null);
          }
        }
      } catch {
        // apiFetch already redirects to /login on a 401; nothing to do here.
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const can = (permission: string) => {
    if (!me) return false;
    // SUPER_ADMIN bypasses every permission check in the backend, so the UI has
    // to agree or it would hide pages that role can actually use.
    if (me.roles.includes("SUPER_ADMIN")) return true;
    return me.permissions.includes(permission);
  };

  return (
    <SessionContext.Provider value={{ me, employee, loading, can }}>{children}</SessionContext.Provider>
  );
}
