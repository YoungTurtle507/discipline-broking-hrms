import type { Metadata } from "next";
import { IBM_Plex_Sans, IBM_Plex_Mono, Manrope } from "next/font/google";
import "./globals.css";

const plexSans = IBM_Plex_Sans({
  variable: "--font-plex-sans",
  weight: ["400", "500", "600", "700"],
  subsets: ["latin"],
  display: "swap",
});

const plexMono = IBM_Plex_Mono({
  variable: "--font-plex-mono",
  weight: ["400", "500", "600", "700"],
  subsets: ["latin"],
  display: "swap",
});

// The login page keeps its own identity (Aurora Split) rather than the
// dashboard's dense Ops Terminal look — Manrope only, scoped to that route.
const manrope = Manrope({
  variable: "--font-manrope",
  weight: ["400", "500", "600", "700", "800"],
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "DB HRMS",
  description: "Secure, contactless attendance and HR management powered by face recognition.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${plexSans.variable} ${plexMono.variable} ${manrope.variable}`}>
      <body>{children}</body>
    </html>
  );
}
