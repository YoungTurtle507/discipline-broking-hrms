import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["192.168.4.36"],
  async redirects() {
    return [
      {
        source: "/",
        destination: "/login",
        permanent: false,
      },
    ];
  },
  // Once the dev server runs over HTTPS (needed for camera access from a LAN
  // IP), calling the plain-HTTP Go backend directly from the browser would be
  // blocked as mixed content. Proxying same-origin through the Next.js server
  // avoids that — the browser only ever talks to this HTTPS origin.
  async rewrites() {
    return [
      {
        // This duplicate's backend runs on 8180 (not 8080 — that's the
        // original HR SYSTEM project, running alongside this one).
        source: "/api/:path*",
        destination: "http://localhost:8180/api/:path*",
      },
    ];
  },
};

export default nextConfig;
