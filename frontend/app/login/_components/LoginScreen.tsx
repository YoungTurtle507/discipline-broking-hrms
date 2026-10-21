"use client";

import { MarketingPanel } from "./MarketingPanel";
import { AuthCard } from "./AuthCard";

// A true side-by-side split, not a card overlaid on a full-width panel — the
// Aurora Split direction the marketing panel and the card each own their own
// share of the screen, so positioning content in either one never has to
// dodge the other.
export function LoginScreen() {
  return (
    <div
      className="flex min-h-screen flex-col overflow-x-hidden lg:h-screen lg:flex-row lg:overflow-hidden"
      style={{ fontFamily: "var(--font-manrope)" }}
    >
      <MarketingPanel />
      <div className="flex flex-1 items-center justify-center bg-[#f4f7fa] px-6 py-10">
        <AuthCard />
      </div>
    </div>
  );
}
