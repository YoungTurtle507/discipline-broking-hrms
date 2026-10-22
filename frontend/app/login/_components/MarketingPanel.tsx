"use client";

import { motion, useReducedMotion, type Variants } from "framer-motion";
import { ShieldCheck, Lightning, UsersThree } from "@phosphor-icons/react";
import { OrbitRing } from "./OrbitRing";
import { OrbitingBadges } from "./OrbitingBadges";

const container: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.1, delayChildren: 0.1 } },
};
const item: Variants = {
  hidden: { opacity: 0, y: 14 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: [0.22, 1, 0.36, 1] } },
};

const HIGHLIGHTS = [
  { icon: ShieldCheck, label: "Secure" },
  { icon: Lightning, label: "Fast" },
  { icon: UsersThree, label: "Simple" },
];

export function MarketingPanel() {
  const reduceMotion = useReducedMotion();

  return (
    <div className="relative min-h-[60vh] overflow-hidden bg-gradient-to-br from-brand-blue-dark via-brand-blue to-navy-light p-[6%] pb-10 text-white lg:h-screen lg:flex-[1.4]">
      <AuroraField reduceMotion={!!reduceMotion} />
      <OrbitingBadges />
      <OrbitRing />

      <motion.div
        variants={container}
        initial={reduceMotion ? false : "hidden"}
        animate="show"
        className="relative z-10"
      >
        <motion.div variants={item} className="flex flex-col items-start gap-2.5">
          {/* Text wordmark until the firm's logo is ready — swap this block
              for an <Image> the same way the rest of the app does once one
              exists. */}
          <div className="flex flex-col leading-[1.05]">
            <span className="text-[1.9rem] font-extrabold tracking-tight sm:text-[2.2rem]">Discipline</span>
            <span className="text-[1.9rem] font-extrabold tracking-tight sm:text-[2.2rem]">Broking</span>
          </div>
          <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-300">
            People &middot; Process &middot; Progress
          </span>
        </motion.div>

        <div className="mt-16 max-w-[520px]">
          <motion.p variants={item} className="mb-3.5 text-[12.5px] font-bold uppercase tracking-[0.18em] text-[#9fc2e8]">
            Smart HR Solutions
          </motion.p>

          <motion.h1 variants={item} className="text-[2.1rem] font-extrabold leading-[1.15] tracking-tight sm:text-[2.5rem]">
            Better People.
            <br />
            <span className="text-[#dbe9f7]">Brighter Tomorrow.</span>
          </motion.h1>

          <motion.p variants={item} className="mt-5 max-w-[400px] text-[15px] leading-relaxed text-[#c3d3e3]">
            Simplify attendance, manage your team, and build a stronger workplace.
          </motion.p>

          <motion.div variants={item} className="mt-7 flex flex-wrap items-center gap-4 text-[13.5px] font-semibold text-[#c3d3e3]">
            {HIGHLIGHTS.map(({ icon: Icon, label }, i) => (
              <span key={label} className="flex items-center gap-4">
                {i > 0 && <span className="h-3.5 w-px bg-white/25" aria-hidden />}
                <span className="flex items-center gap-1.5">
                  <Icon size={17} weight="bold" />
                  {label}
                </span>
              </span>
            ))}
          </motion.div>
        </div>
      </motion.div>
    </div>
  );
}

function AuroraField({ reduceMotion }: { reduceMotion: boolean }) {
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden">
      <div className="absolute -right-16 -top-24 h-[280px] w-[280px] rounded-full bg-[#4a7ba3]/25" />
      <div className="absolute -bottom-40 -left-24 h-[420px] w-[420px] rounded-full bg-[#3a6f9e]/30" />

      <motion.div
        className="absolute -left-[8%] -top-[10%] h-[380px] w-[380px] rounded-full bg-[#4a7ba3] opacity-55 blur-[70px]"
        animate={reduceMotion ? undefined : { x: [0, 40, 0], y: [0, 25, 0] }}
        transition={{ duration: 16, repeat: Infinity, ease: "easeInOut" }}
      />
      <motion.div
        className="absolute -bottom-[15%] -right-[5%] h-[420px] w-[420px] rounded-full bg-[#6fa8d8] opacity-55 blur-[70px]"
        animate={reduceMotion ? undefined : { x: [0, -30, 0], y: [0, -20, 0] }}
        transition={{ duration: 19, repeat: Infinity, ease: "easeInOut" }}
      />
      <motion.div
        className="absolute right-[20%] top-[35%] h-[300px] w-[300px] rounded-full bg-[#e8c77a] opacity-[0.12] blur-[70px]"
        animate={reduceMotion ? undefined : { scale: [1, 1.2, 1] }}
        transition={{ duration: 13, repeat: Infinity, ease: "easeInOut" }}
      />
    </div>
  );
}
