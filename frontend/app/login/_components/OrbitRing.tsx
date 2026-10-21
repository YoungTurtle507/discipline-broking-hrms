"use client";

import { motion, useReducedMotion } from "framer-motion";

// Deliberately not a 3D model — plain CSS circles and Framer Motion rotation.
// A ring, a counter-rotating ring, and a pulsing glow core can't render
// wrong the way freehand 3D geometry can, and they read as "alive" just as
// well through motion alone.
export function OrbitRing() {
  const reduceMotion = useReducedMotion();

  return (
    <div className="pointer-events-none absolute left-[58%] top-[44%] hidden h-[170px] w-[170px] -translate-x-1/2 -translate-y-1/2 sm:block">
      <motion.div
        className="absolute -inset-[22px] rounded-full border border-[#9fc2e8]/30"
        animate={reduceMotion ? undefined : { rotate: -360 }}
        transition={{ duration: 16, repeat: Infinity, ease: "linear" }}
      />
      <motion.div
        className="absolute inset-0 rounded-full border border-[#9fc2e8]/55"
        animate={reduceMotion ? undefined : { rotate: 360 }}
        transition={{ duration: 10, repeat: Infinity, ease: "linear" }}
      >
        <span className="absolute -top-[3px] left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full bg-white" />
        <span className="absolute -bottom-[3px] left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full bg-white" />
      </motion.div>
      <motion.div
        className="absolute inset-[30px] rounded-full border border-white/30"
        style={{
          background: "radial-gradient(circle at 35% 30%, rgba(255,255,255,.35), rgba(159,194,232,.08) 60%, transparent 75%)",
        }}
        animate={reduceMotion ? { opacity: 0.95 } : { scale: [1, 1.06, 1], opacity: [0.9, 1, 0.9] }}
        transition={{ duration: 4, repeat: Infinity, ease: "easeInOut" }}
      />
    </div>
  );
}
