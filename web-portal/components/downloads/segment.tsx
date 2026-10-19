"use client";

import { useRef, type KeyboardEvent, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * A radio group drawn as a segmented control. Keyboard: Tab reaches the checked option,
 * arrow keys move and select (mirrored automatically in right-to-left layouts), Home/End jump.
 */
export function Segment<T extends string>({ label, value, options, onChange, className }: { label: string; value: T | null; options: { id: T; text: string; disabled?: boolean; icon?: ReactNode }[]; onChange: (v: T) => void; className?: string }) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const enabled = options.map((o, i) => (o.disabled ? -1 : i)).filter((i) => i >= 0);
  const current = options.findIndex((o) => o.id === value);
  const focusable = current >= 0 ? current : (enabled[0] ?? 0);

  const onKey = (e: KeyboardEvent<HTMLButtonElement>, i: number) => {
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const fwd = rtl ? "ArrowLeft" : "ArrowRight";
    const back = rtl ? "ArrowRight" : "ArrowLeft";
    const pos = enabled.indexOf(i);
    let next = -1;
    if (e.key === fwd || e.key === "ArrowDown") next = enabled[(pos + 1) % enabled.length];
    else if (e.key === back || e.key === "ArrowUp") next = enabled[(pos - 1 + enabled.length) % enabled.length];
    else if (e.key === "Home") next = enabled[0];
    else if (e.key === "End") next = enabled[enabled.length - 1];
    if (next === undefined || next < 0) return;
    e.preventDefault();
    onChange(options[next].id);
    refs.current[next]?.focus();
  };

  return (
    <div role="radiogroup" aria-label={label} className={cn("track inline-flex max-w-full flex-wrap", className)}>
      {options.map((o, i) => (
        <button
          key={o.id}
          ref={(el) => {
            refs.current[i] = el;
          }}
          type="button"
          role="radio"
          aria-checked={value === o.id}
          disabled={o.disabled}
          tabIndex={i === focusable ? 0 : -1}
          onClick={() => onChange(o.id)}
          onKeyDown={(e) => onKey(e, i)}
          className="tab py-1.5"
        >
          {o.icon}
          {o.text}
        </button>
      ))}
    </div>
  );
}
