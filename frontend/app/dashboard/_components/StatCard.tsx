import Link from "next/link";

type Tone = "success" | "error" | "brand" | "warning" | "accent";

const TONE_TEXT: Record<Tone, string> = {
  success: "text-success",
  error: "text-error",
  brand: "text-brand-blue",
  warning: "text-warning",
  accent: "text-accent-purple",
};

// One cell in the stats-strip — deliberately no icon: the direction reads
// "no icons, no shadows, no rounded cards, hairlines and one accent do
// everything." Cells sit inside a single bordered strip in the parent, so
// this only owns its own right-hand divider.
export function StatCard({
  tone,
  label,
  value,
  percentOfTotal,
  sub,
  href,
  onClick,
  comingSoon,
}: {
  tone: Tone;
  label: string;
  value?: number | string;
  percentOfTotal?: number;
  // Overrides the default "% of total" caption with a real breakdown line
  // (e.g. "2 late · 1 missing checkout") when one is more informative.
  sub?: string;
  href?: string;
  // Drills into the number itself (e.g. "12 absent" -> who those 12 are)
  // rather than navigating away, so it takes priority over href.
  onClick?: () => void;
  comingSoon?: boolean;
}) {
  const className = `min-w-[128px] flex-1 border-r border-slate-200 px-4 py-2.5 text-left transition-colors last:border-r-0 ${
    comingSoon ? "opacity-50" : "hover:bg-slate-50"
  }`;

  const content = (
    <>
      <p className="truncate text-[10px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      {comingSoon ? (
        <>
          <p className="mt-1 font-mono text-xl font-semibold text-slate-300">—</p>
          <p className="mt-0.5 text-[11px] text-slate-400">Coming soon</p>
        </>
      ) : (
        <>
          <p className={`mt-1 font-mono text-xl font-semibold tabular-nums ${TONE_TEXT[tone]}`}>{value}</p>
          <p className="mt-0.5 truncate text-[11px] text-slate-400">{sub ?? `${percentOfTotal}% of total`}</p>
        </>
      )}
    </>
  );

  if (onClick && !comingSoon) {
    return (
      <button type="button" onClick={onClick} className={className}>
        {content}
      </button>
    );
  }
  if (href && !comingSoon) {
    return (
      <Link href={href} className={className}>
        {content}
      </Link>
    );
  }
  return <div className={className}>{content}</div>;
}
