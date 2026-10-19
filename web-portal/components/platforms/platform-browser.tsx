"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useTranslations } from "next-intl";
import { ChevronDown, Layers, Search, X } from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { CategoryIcon, PlatformIcon } from "@/components/platform-icon";
import { Input } from "@/components/ui/input";
import { Link } from "@/i18n/navigation";
import type { CategoryId } from "@/lib/targets";
import { cn } from "@/lib/utils";

export interface PlatformRow {
  id: string;
  name: string;
  total: number;
  auto: number;
  manual: number;
  /** Audited from an exported configuration (--device-config). */
  device: boolean;
  command: string;
}

export interface PlatformCategory {
  id: string;
  groups: { id: string; rows: PlatformRow[] }[];
}

const ALL = "all";

function tabFromHash(hash: string, ids: string[]): string | null {
  const id = hash.replace(/^#/, "");
  return id === ALL || ids.includes(id) ? id : null;
}

/** Searchable, filterable platform browser with one tab per category. Deep links: #database, #network, ?q=postgres. */
export function PlatformBrowser({ categories }: { categories: PlatformCategory[] }) {
  const t = useTranslations("platforms");
  const h = useTranslations("downloads.hardens");
  const ids = useMemo(() => categories.map((c) => c.id), [categories]);
  const [active, setActive] = useState(ALL);
  const [query, setQuery] = useState("");
  const [autoOnly, setAutoOnly] = useState(false);
  const [deviceOnly, setDeviceOnly] = useState(false);
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  useEffect(() => {
    const sync = () => {
      const next = tabFromHash(window.location.hash, ids);
      if (next) setActive(next);
      const q = new URLSearchParams(window.location.search).get("q");
      if (q) setQuery(q);
    };
    sync();
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, [ids]);

  const writeUrl = useCallback((tab: string, q: string) => {
    try {
      const url = new URL(window.location.href);
      if (q) url.searchParams.set("q", q);
      else url.searchParams.delete("q");
      url.hash = tab === ALL ? "" : tab;
      history.replaceState(null, "", url);
    } catch {
      /* some embedded browsers block history changes */
    }
  }, []);

  const select = (tab: string, focus = false) => {
    setActive(tab);
    writeUrl(tab, query);
    if (focus) tabRefs.current[tab]?.focus();
  };

  const onQuery = (q: string) => {
    setQuery(q);
    const tab = q.trim() ? ALL : active;
    setActive(tab);
    writeUrl(tab, q.trim());
  };

  const needle = query.trim().toLowerCase();
  const filtered = useMemo(
    () =>
      categories.map((c) => ({
        id: c.id,
        groups: c.groups
          .map((g) => ({
            id: g.id,
            rows: g.rows.filter(
              (r) =>
                (!needle || r.name.toLowerCase().includes(needle) || r.id.toLowerCase().includes(needle)) &&
                (!autoOnly || r.auto > 0) &&
                (!deviceOnly || r.device),
            ),
          }))
          .filter((g) => g.rows.length > 0),
      })),
    [categories, needle, autoOnly, deviceOnly],
  );
  const stats = (cats: typeof filtered) => {
    const rows = cats.flatMap((c) => c.groups.flatMap((g) => g.rows));
    return { targets: rows.length, rules: rows.reduce((n, r) => n + r.total, 0) };
  };
  const all = stats(filtered);
  const tabs = [{ id: ALL, ...all }, ...filtered.map((c) => ({ id: c.id, ...stats([c]) }))];
  const visible = active === ALL ? filtered : filtered.filter((c) => c.id === active);
  const shown = visible.filter((c) => c.groups.length > 0);

  const onKey = (e: KeyboardEvent<HTMLButtonElement>, i: number) => {
    const last = tabs.length - 1;
    // Arrow keys follow the reading direction, as in the download tabs.
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const fwd = rtl ? "ArrowLeft" : "ArrowRight";
    const back = rtl ? "ArrowRight" : "ArrowLeft";
    const next = e.key === fwd ? (i === last ? 0 : i + 1) : e.key === back ? (i === 0 ? last : i - 1) : e.key === "Home" ? 0 : e.key === "End" ? last : -1;
    if (next >= 0) {
      e.preventDefault();
      select(tabs[next].id, true);
    }
  };

  return (
    <div>
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="relative md:max-w-md md:flex-1">
          <Search className="pointer-events-none absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
          <Input type="search" value={query} onChange={(e) => onQuery(e.target.value)} aria-label={t("searchLabel")} placeholder={t("searchPlaceholder")} className="ps-10 pe-10" />
          {query && (
            <button type="button" onClick={() => onQuery("")} aria-label={t("clear")} className="absolute end-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">
              <X className="h-4 w-4" aria-hidden />
            </button>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" aria-pressed={autoOnly} onClick={() => setAutoOnly((v) => !v)} className="chip">{t("filterAuto")}</button>
          <button type="button" aria-pressed={deviceOnly} onClick={() => setDeviceOnly((v) => !v)} className="chip">{t("filterDevice")}</button>
        </div>
      </div>

      <div role="tablist" aria-label={t("tabs")} className="track mt-5 overflow-x-auto sm:flex-wrap">
        {tabs.map((x, i) => {
          const on = x.id === active;
          return (
            <button
              key={x.id}
              ref={(el) => {
                tabRefs.current[x.id] = el;
              }}
              role="tab"
              type="button"
              id={`tab-${x.id}`}
              aria-selected={on}
              aria-controls="platform-panel"
              tabIndex={on ? 0 : -1}
              onClick={() => select(x.id)}
              onKeyDown={(e) => onKey(e, i)}
              className={cn("tab shrink-0 gap-2.5 text-start", x.targets === 0 && !on && "opacity-50")}
            >
              {x.id === ALL ? <Layers className="h-4 w-4 shrink-0" aria-hidden /> : <CategoryIcon id={x.id as CategoryId} size={16} />}
              <span className="min-w-0">
                <span className="block">{x.id === ALL ? t("tabAll") : h(`categories.${x.id}`)}</span>
                <span className="block text-xs font-normal text-muted-foreground">{h("counts", { targets: x.targets, rules: x.rules })}</span>
              </span>
            </button>
          );
        })}
      </div>

      <div id="platform-panel" role="tabpanel" aria-labelledby={`tab-${active}`} className="mt-2 space-y-4">
        <p className="sr-only" role="status" aria-live="polite">{t("results", { targets: stats(visible).targets })}</p>
        {shown.length === 0 && <p className="glass p-6 text-muted-foreground">{t("noResults")}</p>}
        {shown.map((c) => (
          <details key={`${c.id}-${active}-${needle ? 1 : 0}`} id={`cat-${c.id}`} open={active !== ALL || !!needle} className="glass group/cat scroll-mt-32 [&_summary::-webkit-details-marker]:hidden">
            <summary className="flex cursor-pointer list-none items-start gap-3 p-4 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary group-open/cat:border-b group-open/cat:border-border">
              <CategoryIcon id={c.id as CategoryId} size={40} tile />
              <div className="min-w-0">
                <h2 className="text-lg font-semibold">{h(`categories.${c.id}`)}</h2>
                <p className="mt-1 text-sm text-muted-foreground">{h(`categoryIntro.${c.id}`)}</p>
              </div>
              <ChevronDown className="ms-auto mt-1 h-5 w-5 shrink-0 text-muted-foreground transition-transform group-open/cat:rotate-180" aria-hidden />
            </summary>
            <div className="space-y-6 p-4">
              {c.groups.map((g) => (
                <div key={g.id}>
                  <h3 className="label-caps mb-2 text-accent-text">{h(`groups.${g.id}`)}</h3>
                  <ul className="divide-y divide-border rounded-lg border border-border">
                    {g.rows.map((r) => (
                      <li key={r.id}>
                        <details className="group [&_summary::-webkit-details-marker]:hidden">
                          <summary className="flex cursor-pointer list-none items-center gap-3 p-3 transition-colors hover:bg-muted/30 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary">
                            <PlatformIcon id={r.id} category={c.id as CategoryId} size={32} tile />
                            <div className="min-w-0 flex-1">
                              <p className="font-medium" lang="en">{r.name}</p>
                              <p className="text-xs text-muted-foreground">{h("ruleCounts", { total: r.total, auto: r.auto, manual: r.manual })}</p>
                            </div>
                            <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden />
                          </summary>
                          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-border bg-muted/20 p-3">
                            <div className="flex min-w-0 flex-1 basis-80 items-center gap-2">
                              <code className="min-w-0 flex-1 break-all rounded-md bg-code px-2 py-1.5 font-mono text-xs text-slate-200" dir="ltr">{r.command}</code>
                              <CopyButton text={r.command} label={h("copyCommandFor", { name: r.name })} compact tone="themed" />
                            </div>
                            <Link href={`/rules?target=${encodeURIComponent(r.id)}`} className="link text-sm" aria-label={h("viewRulesFor", { name: r.name })}>
                              {h("viewRules")}
                            </Link>
                          </div>
                        </details>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </details>
        ))}
      </div>
    </div>
  );
}
