"use client";

import { useEffect, useId, useState } from "react";
import { useRouter } from "next/navigation";
import { motion, AnimatePresence, useMotionValue, useReducedMotion, useSpring } from "framer-motion";
import {
  EnvelopeSimple,
  LockKey,
  Eye,
  EyeSlash,
  ArrowRight,
  CheckCircle,
  CircleNotch,
  WarningCircle,
} from "@phosphor-icons/react";
import { getBackendUrl } from "@/lib/api";

// A few degrees of "magnetic" pull toward the cursor on hover — the button
// nudges toward you rather than sitting dead still. Disabled states (mid
// submit, already succeeded) skip it so the button doesn't drift underneath
// a spinner or checkmark the user is trying to read.
function useMagnetic(disabled: boolean) {
  const x = useMotionValue(0);
  const y = useMotionValue(0);
  const springX = useSpring(x, { stiffness: 200, damping: 15 });
  const springY = useSpring(y, { stiffness: 200, damping: 15 });

  return {
    style: { x: springX, y: springY },
    onMouseMove: (e: React.MouseEvent<HTMLButtonElement>) => {
      if (disabled) return;
      const rect = e.currentTarget.getBoundingClientRect();
      x.set((e.clientX - rect.left - rect.width / 2) * 0.15);
      y.set((e.clientY - rect.top - rect.height / 2) * 0.3);
    },
    onMouseLeave: () => {
      x.set(0);
      y.set(0);
    },
  };
}

// Only the email/username is ever kept in the browser — never the password.
// Storing a password in plain text in localStorage would put it wherever
// that browser profile is used (readable by any script or extension with
// page access, unlike a password itself), for a feature that already has a
// secure equivalent: the field below is marked autoComplete="current-password"
// specifically so the browser's OWN password manager can offer to remember
// it, encrypted and tied to the OS/browser profile.
const REMEMBERED_EMAIL_KEY = "hrms.rememberedEmail";

export function AuthCard() {
  const router = useRouter();
  const reduceMotion = useReducedMotion();
  const emailId = useId();
  const passwordId = useId();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const magnetic = useMagnetic(submitting || success);

  // Pre-fill from a previous "Remember me" — wrapped in try/catch since
  // localStorage can throw in a private window or with storage blocked.
  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(REMEMBERED_EMAIL_KEY);
      if (saved) {
        setEmail(saved);
        setRemember(true);
      }
    } catch {
      // Nothing saved to restore — the form just starts blank.
    }
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      const res = await fetch(`${getBackendUrl()}/api/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username: email, password }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error ?? "Sign in failed. Please try again.");
        setSubmitting(false);
        return;
      }

      try {
        if (remember) {
          window.localStorage.setItem(REMEMBERED_EMAIL_KEY, email);
        } else {
          window.localStorage.removeItem(REMEMBERED_EMAIL_KEY);
        }
      } catch {
        // Storage unavailable — sign-in itself already succeeded regardless.
      }

      // A beat of "Signed in" before the route change reads as confirmation
      // rather than the page just abruptly vanishing.
      setSubmitting(false);
      setSuccess(true);
      setTimeout(() => router.push("/dashboard"), reduceMotion ? 150 : 650);
    } catch {
      setError("Can't reach the server. Is the backend running?");
      setSubmitting(false);
    }
  }

  return (
    <motion.div
      initial={reduceMotion ? false : { opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.55, ease: [0.22, 1, 0.36, 1], delay: 0.2 }}
      className="relative z-10 w-full max-w-[420px] rounded-[24px] border border-slate-200/80 bg-white p-8 shadow-[0_30px_70px_-25px_rgba(15,23,42,0.28)] sm:p-9"
    >
        <motion.svg
          width="52"
          height="5"
          viewBox="0 0 52 5"
          className="mb-5"
          initial={reduceMotion ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.35, duration: 0.4 }}
        >
          <defs>
            <linearGradient id="dashGradient" x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor="#234a6e" />
              <stop offset="100%" stopColor="#2d5d8b" />
            </linearGradient>
          </defs>
          <rect width="52" height="5" rx="2.5" fill="url(#dashGradient)" />
        </motion.svg>

        <h1 className="text-[26px] font-bold tracking-tight text-slate-900 sm:text-[28px]">
          Welcome
        </h1>
        <p className="mt-1.5 text-sm leading-relaxed text-slate-500">
          Sign in to continue to your HRMS account
        </p>

        <form onSubmit={handleSubmit} className="mt-7 flex flex-col gap-3.5" noValidate>
          <div>
            <label htmlFor={emailId} className="sr-only">
              Work email
            </label>
            <div className="flex items-center gap-3 rounded-[14px] bg-slate-50 px-3.5 py-3 ring-1 ring-slate-200 transition-shadow focus-within:ring-2 focus-within:ring-brand-blue">
              <EnvelopeSimple size={18} className="shrink-0 text-slate-400" />
              <input
                id={emailId}
                type="email"
                name="email"
                autoComplete="email"
                spellCheck={false}
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="Work Email"
                className="w-full bg-transparent text-[15px] text-slate-800 outline-none placeholder:text-slate-400"
              />
            </div>
          </div>

          <div>
            <label htmlFor={passwordId} className="sr-only">
              Password
            </label>
            <div className="flex items-center gap-3 rounded-[14px] bg-slate-50 px-3.5 py-3 ring-1 ring-slate-200 transition-shadow focus-within:ring-2 focus-within:ring-brand-blue">
              <LockKey size={18} className="shrink-0 text-slate-400" />
              <input
                id={passwordId}
                type={showPassword ? "text" : "password"}
                name="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Password"
                className="w-full bg-transparent text-[15px] text-slate-800 outline-none placeholder:text-slate-400"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? "Hide password" : "Show password"}
                className="shrink-0 rounded text-slate-400 hover:text-slate-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-blue"
              >
                <AnimatePresence mode="wait" initial={false}>
                  <motion.span
                    key={showPassword ? "on" : "off"}
                    initial={reduceMotion ? false : { opacity: 0, scale: 0.7 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={reduceMotion ? undefined : { opacity: 0, scale: 0.7 }}
                    transition={{ duration: 0.12 }}
                    className="flex"
                  >
                    {showPassword ? <EyeSlash size={18} /> : <Eye size={18} />}
                  </motion.span>
                </AnimatePresence>
              </button>
            </div>
          </div>

          <AnimatePresence>
            {error && (
              <motion.div
                initial={reduceMotion ? false : { opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                exit={reduceMotion ? undefined : { opacity: 0, height: 0 }}
                transition={{ duration: 0.2 }}
                className="flex items-center gap-2 overflow-hidden rounded-[10px] bg-red-50 px-3.5 py-2.5 text-[13px] text-red-600"
                role="alert"
              >
                <WarningCircle size={16} weight="fill" className="shrink-0" />
                {error}
              </motion.div>
            )}
          </AnimatePresence>

          <div className="flex items-center justify-between text-[13.5px]">
            <label className="flex items-center gap-2 text-slate-500">
              <input
                type="checkbox"
                checked={remember}
                onChange={(e) => setRemember(e.target.checked)}
                className="h-4 w-4 rounded-[5px] border-slate-300 accent-navy focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-blue"
              />
              Remember me
            </label>
            <a
              href="#"
              className="rounded font-semibold text-brand-blue hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-blue"
            >
              Forgot password?
            </a>
          </div>

          <motion.button
            {...magnetic}
            style={magnetic.style}
            whileHover={reduceMotion || submitting || success ? undefined : { scale: 1.01 }}
            whileTap={reduceMotion || submitting || success ? undefined : { scale: 0.98 }}
            type="submit"
            disabled={submitting || success}
            aria-live="polite"
            className={`group mt-1 flex items-center justify-center gap-2 rounded-[14px] py-3.5 text-[14.5px] font-semibold text-white shadow-[0_14px_26px_-10px_rgba(35,74,110,0.55)] transition-[background,box-shadow] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-blue disabled:opacity-100 ${
              success ? "bg-success" : "bg-gradient-to-r from-brand-blue-dark to-brand-blue hover:shadow-[0_18px_32px_-10px_rgba(35,74,110,0.65)]"
            }`}
          >
            <AnimatePresence mode="wait" initial={false}>
              {success ? (
                <motion.span
                  key="success"
                  initial={{ opacity: 0, scale: 0.7 }}
                  animate={{ opacity: 1, scale: 1 }}
                  className="flex items-center gap-2"
                >
                  <CheckCircle size={18} weight="fill" />
                  Signed in
                </motion.span>
              ) : submitting ? (
                <motion.span key="submitting" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex items-center gap-2">
                  <motion.span
                    animate={reduceMotion ? undefined : { rotate: 360 }}
                    transition={{ repeat: Infinity, duration: 0.7, ease: "linear" }}
                    className="flex"
                  >
                    <CircleNotch size={17} weight="bold" />
                  </motion.span>
                  Signing in&hellip;
                </motion.span>
              ) : (
                <motion.span key="idle" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="flex items-center gap-2">
                  Sign In
                  <ArrowRight size={17} weight="bold" className="transition-transform group-hover:translate-x-1" />
                </motion.span>
              )}
            </AnimatePresence>
          </motion.button>
        </form>

        <div className="my-5 h-px w-full bg-slate-200" />

        <p className="text-center text-[13.5px] text-slate-500">
          Don&apos;t have an account?{" "}
          <a href="#" className="font-semibold text-brand-blue hover:underline">
            Contact your workspace admin.
          </a>
        </p>
    </motion.div>
  );
}
