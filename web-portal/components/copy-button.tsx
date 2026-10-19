"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

export function CopyButton({ text, label, className, compact, tone = "dark" }: { text: string; label?: string; className?: string; compact?: boolean; tone?: "dark" | "themed" }) {
  const t = useTranslations("common");
  const [done, setDone] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setDone(true);
      setTimeout(() => setDone(false), 1800);
    } catch {
      /* clipboard blocked: nothing to do */
    }
  };
  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border px-2 text-xs font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary",
        tone === "dark" ? "border-white/15 bg-white/5 text-slate-200 hover:bg-white/10" : "border-border-strong bg-card text-foreground hover:border-primary hover:bg-muted",
        compact && "w-8 justify-center px-0",
        className,
      )}
      aria-label={label ?? t("copy")}
    >
      {done ? <Check className="h-3.5 w-3.5 text-emerald-400" aria-hidden /> : <Copy className="h-3.5 w-3.5" aria-hidden />}
      <span aria-live="polite" className={compact ? "sr-only" : undefined}>{done ? t("copied") : t("copy")}</span>
    </button>
  );
}
