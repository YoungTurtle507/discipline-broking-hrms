"use client";

import { useRef } from "react";

function toDisplay(iso: string): string {
  const [y, m, d] = iso.split("-");
  if (!y || !m || !d) return "";
  return `${d}/${m}/${y}`;
}

type DateFieldProps = {
  value: string;
  onChange: (iso: string) => void;
  className?: string;
  required?: boolean;
  min?: string;
  max?: string;
  id?: string;
  disabled?: boolean;
};

// A native <input type="date"> always carries an ISO yyyy-mm-dd value, but
// what it *displays* when unfocused follows the browser/OS locale — which can
// silently render MM/DD/YYYY for a US-locale browser. There's no attribute to
// force a display format, so this hides the native text and draws our own
// DD/MM/YYYY label on top, while the real input underneath still handles the
// calendar picker, keyboard entry, and native validation.
export function DateField({ value, onChange, className, required, min, max, id, disabled }: DateFieldProps) {
  const ref = useRef<HTMLInputElement>(null);
  return (
    <div className="relative">
      <input
        ref={ref}
        id={id}
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        min={min}
        max={max}
        disabled={disabled}
        onClick={() => ref.current?.showPicker?.()}
        className={className}
        // -webkit-text-fill-color (not `color`) hides just the native date
        // text under our overlay — Chrome's calendar-icon glyph is drawn from
        // `color`, so setting `color` itself transparent blanks the icon too.
        style={{ colorScheme: "light", WebkitTextFillColor: "transparent" }}
      />
      <span
        className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-sm"
        style={{ color: value ? "#0f172a" : "#94a3b8" }}
      >
        {value ? toDisplay(value) : "dd/mm/yyyy"}
      </span>
    </div>
  );
}
