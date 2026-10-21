"use client";

import { motion, useReducedMotion } from "framer-motion";
import { UsersThree, ShieldCheck, CheckCircle, Clock } from "@phosphor-icons/react";

// Glass-morphism capability badges floating around the orbit ring — positions
// and float timing measured off the Aurora Split mockup.
const BADGES = [
  { icon: UsersThree, pos: "left-[52%] top-[16%]", float: 6, delay: 0 },
  { icon: ShieldCheck, pos: "left-[68%] top-[38%]", float: 7, delay: 1.1 },
  { icon: CheckCircle, pos: "left-[46%] top-[56%]", float: 6.5, delay: 0.5 },
  { icon: Clock, pos: "left-[66%] top-[70%]", float: 7.5, delay: 1.6 },
];

function Badge({
  Icon,
  float,
  delay,
}: {
  Icon: React.ComponentType<{ size?: number; weight?: "fill" | "regular" | "bold" }>;
  float: number;
  delay: number;
}) {
  const reduceMotion = useReducedMotion();
  return (
    <motion.div
      className="flex h-14 w-14 items-center justify-center rounded-2xl border border-white/25 bg-white/10 text-white shadow-[0_12px_28px_-8px_rgba(0,0,20,0.5)] backdrop-blur-md"
      initial={reduceMotion ? false : { opacity: 0, scale: 0.7 }}
      animate={
        reduceMotion
          ? { opacity: 1, scale: 1 }
          : { opacity: 1, scale: 1, y: [0, -10, 0] }
      }
      transition={{
        opacity: { duration: 0.6, delay: 0.5 + delay },
        scale: { duration: 0.6, delay: 0.5 + delay },
        y: { duration: float, repeat: Infinity, ease: "easeInOut", delay: 0.5 + delay },
      }}
    >
      <Icon size={24} weight="fill" />
    </motion.div>
  );
}

export function OrbitingBadges() {
  return (
    <div className="pointer-events-none absolute inset-0 hidden sm:block">
      {BADGES.map(({ icon: Icon, pos, float, delay }, i) => (
        <div key={i} className={`absolute -translate-x-1/2 -translate-y-1/2 ${pos}`}>
          <Badge Icon={Icon} float={float} delay={delay} />
        </div>
      ))}
    </div>
  );
}
