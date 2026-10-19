"use client";

import { useSyncExternalStore } from "react";
import { useTranslations } from "next-intl";
import { Monitor, Moon, Sun } from "lucide-react";

type Theme = "system" | "dark" | "light";
const ORDER: Theme[] = ["system", "dark", "light"];
const EVENT = "tma-theme";

// The <html data-theme> attribute (set before first paint by the inline script) is the single source of truth.
function snapshot(): Theme {
  const v = document.documentElement.dataset.theme;
  return v === "dark" || v === "light" ? v : "system";
}
function subscribe(cb: () => void) {
  window.addEventListener(EVENT, cb);
  return () => window.removeEventListener(EVENT, cb);
}

/** Cycles system / dark / light. The choice lives in localStorage (not a cookie). */
export function ThemeToggle() {
  const t = useTranslations("nav.theme");
  const theme = useSyncExternalStore(subscribe, snapshot, () => "system" as Theme);

  const next = () => {
    const n = ORDER[(ORDER.indexOf(theme) + 1) % ORDER.length];
    if (n === "system") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.dataset.theme = n;
    try {
      if (n === "system") localStorage.removeItem("tma-theme");
      else localStorage.setItem("tma-theme", n);
    } catch {
      /* private mode: the choice just is not remembered */
    }
    window.dispatchEvent(new Event(EVENT));
  };
  const Icon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;
  return (
    <button
      type="button"
      onClick={next}
      className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-card text-foreground hover:border-primary"
      aria-label={`${t("label")}: ${t(theme)}`}
      title={`${t("label")}: ${t(theme)}`}
    >
      <Icon className="h-4 w-4" aria-hidden />
    </button>
  );
}
