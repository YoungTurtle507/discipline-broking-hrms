type HealthStatus = {
  status: string;
  database: string;
  cache: string;
};

async function getBackendHealth(): Promise<
  { ok: true; data: HealthStatus } | { ok: false; error: string }
> {
  const url = `${process.env.BACKEND_INTERNAL_URL ?? "http://backend:8080"}/health`;
  try {
    const res = await fetch(url, { cache: "no-store" });
    const data = (await res.json()) as HealthStatus;
    return { ok: true, data };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : "unknown error" };
  }
}

export default async function StatusPage() {
  const health = await getBackendHealth();

  return (
    <main style={{ fontFamily: "sans-serif", padding: "2rem", maxWidth: 480 }}>
      <h1>HR System — Foundation</h1>
      <p>Live status pulled server-side from the Go backend on every request:</p>

      {health.ok ? (
        <ul>
          <li>Backend: {health.data.status}</li>
          <li>Database (Postgres): {health.data.database}</li>
          <li>Cache (Redis): {health.data.cache}</li>
        </ul>
      ) : (
        <p style={{ color: "crimson" }}>
          Could not reach backend: {health.error}
        </p>
      )}
    </main>
  );
}
