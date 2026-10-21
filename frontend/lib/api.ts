// Requests go same-origin through the Next.js dev server, which proxies
// /api/* to the Go backend (see next.config.ts rewrites). This keeps the
// browser talking to one HTTPS origin — calling the backend's own http://
// host:port directly would be blocked as mixed content once the frontend
// runs over HTTPS (required for camera access from a LAN IP).
export function getBackendUrl(): string {
  return process.env.NEXT_PUBLIC_BACKEND_URL ?? "";
}

export class ApiError extends Error {
  status: number;
  // The full parsed error body, for callers that need more than the message
  // — e.g. a structured flag like `reason_required` alongside an employee id.
  body?: Record<string, unknown>;
  constructor(message: string, status: number, body?: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

// A session can end mid-use: it expires, an admin deactivates the account, or
// an admin resets the password (which invalidates sessions issued before it).
// Send the user to the login page instead of letting every caller surface a
// bare "session expired" as an unhandled rejection.
function redirectToLogin() {
  if (typeof window === "undefined") return;
  if (window.location.pathname.startsWith("/login")) return;
  // A full page load, not router.push: the session is gone, so every bit of
  // cached client state is now wrong and should be discarded. useRouter isn't
  // available here anyway — this is a plain module, not a component.
  // eslint-disable-next-line @next/next/no-location-assign-relative-destination
  window.location.assign("/login");
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${getBackendUrl()}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    // The login request itself returns 401 for a wrong password — that belongs
    // to the login form, not a redirect.
    if (res.status === 401 && !path.startsWith("/api/v1/auth/login")) {
      redirectToLogin();
    }
    throw new ApiError(data?.error ?? `Request failed (${res.status})`, res.status, data ?? undefined);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// For multipart uploads — no Content-Type header, so the browser sets its
// own with the correct boundary.
export async function apiUpload<T>(path: string, formData: FormData): Promise<T> {
  const res = await fetch(`${getBackendUrl()}${path}`, {
    method: "POST",
    credentials: "include",
    body: formData,
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    if (res.status === 401) {
      redirectToLogin();
    }
    throw new ApiError(data?.error ?? `Request failed (${res.status})`, res.status, data ?? undefined);
  }
  return res.json() as Promise<T>;
}
