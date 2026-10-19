"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ChevronDown, ExternalLink, FlaskConical, Info, Search, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { CodeBlock } from "@/components/code-block";
import { CopyButton } from "@/components/copy-button";
import { PlatformIcon } from "@/components/platform-icon";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Link } from "@/i18n/navigation";
import type { GuideMode, GuideRow } from "@/lib/device-guides";
import { cn } from "@/lib/utils";

type Filter = "all" | GuideMode;
// Same order as GUIDE_MODES in lib/device-guides.ts (kept here so the guide table is not bundled for the browser).
const PREVIEW = 12;
const FILTERS: Filter[] = ["all", "host", "device-config", "cloud", "k8s", "manual"];

/** `#devices-cloud` selects a filter; `#devices` (or anything else) leaves it alone. */
function filterFromHash(hash: string): Filter | null {
  const m = hash.match(/^#devices-(.+)$/);
  return m && (FILTERS as string[]).includes(m[1]) ? (m[1] as Filter) : null;
}

const MODE_BADGE: Record<GuideMode, "primary" | "accent" | "default" | "medium"> = { host: "primary", "device-config": "accent", cloud: "primary", k8s: "primary", manual: "default" };

function Fold({ title, icon, children }: { title: string; icon?: ReactNode; children: ReactNode }) {
  return (
    <details className="group rounded-lg border border-border bg-muted/30 [&_summary::-webkit-details-marker]:hidden">
      <summary className="flex cursor-pointer list-none items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary">
        {icon}
        <span className="min-w-0 flex-1">{title}</span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden />
      </summary>
      <div className="space-y-3 border-t border-border p-3">{children}</div>
    </details>
  );
}

/** A command that wraps instead of scrolling, with a copy button. */
function CommandLine({ code, label }: { code: string; label: string }) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <code className="min-w-0 flex-1 break-all rounded-md bg-code px-2 py-1.5 font-mono text-xs text-slate-200" dir="ltr">{code}</code>
      <CopyButton text={code} label={label} compact tone="themed" />
    </div>
  );
}

function GuideCard({ row, githubUrl }: { row: GuideRow; githubUrl: string }) {
  const t = useTranslations("downloads.devices");
  const h = useTranslations("downloads.hardens");
  const prepTitle = row.mode === "device-config" ? t("exportTitle") : row.mode === "cloud" ? t("loginTitle") : row.mode === "k8s" ? t("kubeTitle") : t("prepTitle");
  const hasPrep = (row.how?.length ?? 0) > 0 || !!row.ui;
  const ref = useRef<HTMLDetailsElement>(null);
  // A deep link (#device-<id>) opens its card.
  useEffect(() => {
    const open = () => {
      if (window.location.hash === `#device-${row.id}`) ref.current?.setAttribute("open", "");
    };
    open();
    window.addEventListener("hashchange", open);
    return () => window.removeEventListener("hashchange", open);
  }, [row.id]);
  const lang = row.howLang === "powershell" ? "powershell" : row.howLang === "cli" ? "device cli" : "shell";

  return (
    <details id={`device-${row.id}`} ref={ref} className="glass group min-w-0 scroll-mt-32 [&_summary::-webkit-details-marker]:hidden">
      <summary className="flex cursor-pointer list-none flex-wrap items-start gap-3 rounded-[inherit] p-4 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary">
        <PlatformIcon id={row.id} category={row.category} size={36} tile />
        <div className="min-w-0 flex-1">
          <h3 id={`device-${row.id}-h`} className="text-base font-semibold" lang="en">{row.name}</h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            <code className="font-mono" dir="ltr">{row.id}</code> · {h("ruleCounts", { total: row.total, auto: row.auto, manual: row.manual })}
          </p>
        </div>
        <Badge variant={MODE_BADGE[row.mode]}>{t(`modes.${row.mode}.label`)}</Badge>
        <ChevronDown className="mt-1 h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden />
      </summary>
      <div className="flex flex-col gap-3 border-t border-border p-4">

      <p className="text-sm">
        <span className="font-medium">{t("whereTitle")}:</span> <span className="text-muted-foreground">{t(`where.${row.where}`)}</span>
      </p>
      {row.mode === "manual" && <p className="text-sm text-muted-foreground">{t("manualText")}</p>}
      {row.note && (
        <p className="flex items-start gap-2 text-sm text-muted-foreground">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden />
          <span>
            {t(`notes.${row.note}`)}
            {row.note === "experimental" && (
              <>
                {" "}
                <a href="#os-unix" className="link">{t("experimentalLink")}</a>
              </>
            )}
          </span>
        </p>
      )}

      {hasPrep && (
        <Fold title={prepTitle}>
          {row.ui && (
            <p className="text-sm">
              <span className="font-medium">{t("uiTitle")}:</span>{" "}
              <code className="break-words rounded bg-muted px-1.5 py-0.5 font-mono text-xs" dir="ltr" lang="en">{row.ui}</code>
            </p>
          )}
          {row.how && row.how.length > 0 && <CodeBlock code={row.how.join("\n")} lang={lang} />}
          {row.mode === "device-config" && <p className="text-xs text-muted-foreground">{t("saveFile")}</p>}
          {row.verify && (
            <p className="flex items-start gap-2 rounded-md border border-warning/50 bg-warning/10 p-2 text-xs">
              <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-warning" aria-hidden />
              <span>{t("verify")}</span>
            </p>
          )}
        </Fold>
      )}
      {!hasPrep && row.verify && <p className="text-xs text-muted-foreground">{t("verify")}</p>}

      <div className="min-w-0">
        <p className="label-caps mb-1.5">{t("scanTitle")}</p>
        <CommandLine code={row.command} label={h("copyCommandFor", { name: row.name })} />
      </div>

      {row.sample && (
        <Fold title={t("sample.title")} icon={<FlaskConical className="h-4 w-4 shrink-0 text-accent-text" aria-hidden />}>
          {row.device > 0 ? (
            <>
              <p className="text-sm text-muted-foreground">{t("sample.text")}</p>
              <CommandLine code={row.sample.command} label={t("sample.title")} />
              <p className="text-xs text-muted-foreground">{t("sample.windows")}</p>
              <CommandLine code={row.sample.windowsCommand} label={`${t("sample.title")} (Windows)`} />
            </>
          ) : (
            <p className="text-sm text-muted-foreground">{t("sample.noDevice")}</p>
          )}
          <a href={`${githubUrl}/tree/main/${row.sample.path}`} target="_blank" rel="noopener noreferrer" className="link inline-flex items-center gap-1 text-sm">
            {t("sample.link")}
            <ExternalLink className="h-3.5 w-3.5" aria-hidden />
          </a>
        </Fold>
      )}

      <p className="mt-auto pt-1 text-sm">
        <Link href={`/rules?target=${encodeURIComponent(row.id)}`} className="link" aria-label={h("viewRulesFor", { name: row.name })}>
          {t("rulesLink")}
        </Link>
      </p>
      </div>
    </details>
  );
}

/** Searchable, filterable "Test your device" guide. Deep links: #devices-device-config, #devices-cloud, ?q=cisco. */
export function DevicesBrowser({ rows, githubUrl }: { rows: GuideRow[]; githubUrl: string }) {
  const t = useTranslations("downloads.devices");
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  const [all, setAll] = useState(false);

  useEffect(() => {
    const sync = () => {
      const f = filterFromHash(window.location.hash);
      if (f) setFilter(f);
      if (/^#device-/.test(window.location.hash)) setAll(true);
      const q = new URLSearchParams(window.location.search).get("q");
      if (q) setQuery(q);
    };
    sync();
    // Content above this section (detection, release cards) grows after hydration, so the browser's own jump
    // to #devices or #device-<id> can land too high: jump again once the layout has settled.
    const hash = window.location.hash;
    const frame = /^#devices?-|^#devices$/.test(hash) ? window.setTimeout(() => document.getElementById(hash.slice(1))?.scrollIntoView(), 300) : 0;
    window.addEventListener("hashchange", sync);
    return () => {
      window.clearTimeout(frame);
      window.removeEventListener("hashchange", sync);
    };
  }, []);

  const writeUrl = useCallback((f: Filter, q: string) => {
    try {
      const url = new URL(window.location.href);
      if (q) url.searchParams.set("q", q);
      else url.searchParams.delete("q");
      url.hash = f === "all" ? "devices" : `devices-${f}`;
      history.replaceState(null, "", url);
    } catch {
      /* some embedded browsers block history changes */
    }
  }, []);

  const needle = query.trim().toLowerCase();
  const matches = useMemo(() => rows.filter((r) => !needle || r.name.toLowerCase().includes(needle) || r.id.includes(needle)), [rows, needle]);
  const counts = useMemo(() => {
    const c: Record<Filter, number> = { all: matches.length, host: 0, "device-config": 0, cloud: 0, k8s: 0, manual: 0 };
    for (const r of matches) c[r.mode]++;
    return c;
  }, [matches]);
  const filtered = filter === "all" ? matches : matches.filter((r) => r.mode === filter);
  // Browsing without a search or filter starts with a short list so the page stays compact.
  const shown = all || needle || filter !== "all" ? filtered : filtered.slice(0, PREVIEW);

  const choose = (f: Filter) => {
    setFilter(f);
    writeUrl(f, needle);
  };
  const onQuery = (q: string) => {
    setQuery(q);
    writeUrl(filter, q.trim());
  };

  return (
    <div>
      <div className="flex flex-col gap-3">
        <div className="relative md:max-w-md">
          <Search className="pointer-events-none absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
          <Input
            type="search"
            value={query}
            onChange={(e) => onQuery(e.target.value)}
            aria-label={t("searchLabel")}
            aria-controls="devices-list"
            placeholder={t("searchPlaceholder")}
            className="ps-10 pe-10"
          />
          {query && (
            <button type="button" onClick={() => onQuery("")} aria-label={t("clear")} className="absolute end-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">
              <X className="h-4 w-4" aria-hidden />
            </button>
          )}
        </div>
        <div role="group" aria-label={t("filterLabel")} className="flex flex-wrap gap-2">
          {FILTERS.map((f) => {
            const on = f === filter;
            return (
              <button
                key={f}
                id={`devices-${f}`}
                type="button"
                aria-pressed={on}
                aria-controls="devices-list"
                onClick={() => choose(f)}
                className={cn("chip scroll-mt-36", counts[f] === 0 && !on && "opacity-60")}
              >
                {t(`modes.${f}.label`)}
                <span className="chip-count">{counts[f]}</span>
              </button>
            );
          })}
        </div>
        {filter !== "all" && <p className="text-sm text-muted-foreground">{t(`modes.${filter}.text`)}</p>}
      </div>

      <p className="sr-only" role="status" aria-live="polite">{t("results", { count: shown.length })}</p>
      <div id="devices-list" className="mt-5">
        {shown.length === 0 ? (
          <p className="glass p-6 text-muted-foreground">{t("noResults")}</p>
        ) : (
          <div className="grid gap-4 lg:grid-cols-2">
            {shown.map((r) => <GuideCard key={r.id} row={r} githubUrl={githubUrl} />)}
          </div>
        )}
        {shown.length < filtered.length && (
          <button type="button" onClick={() => setAll(true)} className="chip mt-4">
            {t("showAll", { count: filtered.length })}
          </button>
        )}
      </div>
    </div>
  );
}
