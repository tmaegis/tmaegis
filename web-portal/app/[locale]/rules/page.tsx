import type { Metadata } from "next";
import { ArrowLeft, ArrowRight, Search } from "lucide-react";
import { getFormatter, getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { PlatformIcon } from "@/components/platform-icon";
import { SeverityBadge } from "@/components/severity-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { Link } from "@/i18n/navigation";
import { normalizeQuery, searchRules } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { TARGETS } from "@/lib/targets";

/** Quick target filters above the form (the select lists every target). */
const POPULAR = ["linux", "windows", "macos", "docker", "kubernetes", "postgres", "nginx", "aws", "azure", "cisco-ios", "github", "m365"];

type SP = Promise<Record<string, string | string[] | undefined>>;

export async function generateMetadata({ params, searchParams }: { params: Promise<{ locale: string }>; searchParams: SP }): Promise<Metadata> {
  const { locale } = await params;
  const sp = await searchParams;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({
    locale,
    path: "/rules",
    title: t("pages.rules"),
    description: t("descriptions.rules"),
    ogTitleEn: "Rule explorer",
    noindex: Object.keys(sp).length > 0,
  });
}

export default async function Rules({ params, searchParams }: { params: Promise<{ locale: string }>; searchParams: SP }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("rules");
  const fmt = await getFormatter();
  const q = normalizeQuery(await searchParams);
  const res = searchRules(q);

  const href = (page: number) => {
    const p = new URLSearchParams();
    if (q.q) p.set("q", q.q);
    if (q.target) p.set("target", q.target);
    if (q.severity) p.set("severity", q.severity);
    if (q.level) p.set("level", q.level);
    if (q.type) p.set("type", q.type);
    if (page > 1) p.set("page", String(page));
    const s = p.toString();
    return `/rules${s ? `?${s}` : ""}`;
  };
  const pageNumbers: number[] = [];
  for (let n = Math.max(1, res.page - 2); n <= Math.min(res.pages, res.page + 2); n++) pageNumbers.push(n);
  const filtered = !!(q.q || q.target || q.severity || q.level || q.type);
  const label = "mb-1.5 block text-sm font-medium";

  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        <nav aria-label={t("popular")} className="-mx-4 mb-4 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0">
          <ul className="flex gap-2 sm:flex-wrap">
            {POPULAR.filter((id) => res.facets.targets.some((x) => x.id === id)).map((id) => {
              const on = q.target === id;
              return (
                <li key={id}>
                  <Link href={on ? "/rules" : `/rules?target=${encodeURIComponent(id)}`} aria-current={on ? "true" : undefined} className="chip group" lang="en">
                    <PlatformIcon id={id} category={TARGETS[id]?.category} size={16} active={on} />
                    {TARGETS[id]?.name ?? id}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
        <form method="get" action={`/${locale}/rules`} role="search" aria-label={t("searchLabel")} className="glass grid gap-4 p-4 sm:grid-cols-2 sm:p-5 lg:grid-cols-6">
          <div className="sm:col-span-2 lg:col-span-2">
            <label htmlFor="q" className={label}>{t("searchLabel")}</label>
            <div className="relative">
              <Search className="pointer-events-none absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
              <Input id="q" name="q" type="search" defaultValue={q.q} placeholder={t("searchPlaceholder")} className="ps-9" maxLength={100} autoComplete="off" />
            </div>
          </div>
          <div>
            <label htmlFor="target" className={label}>{t("target")}</label>
            <Select id="target" name="target" defaultValue={q.target}>
              <option value="">{t("allTargets")}</option>
              {res.facets.targets.map((x) => (
                <option key={x.id} value={x.id}>{TARGETS[x.id] ? `${TARGETS[x.id].name} · ${x.id}` : x.id} ({fmt.number(x.count)})</option>
              ))}
            </Select>
          </div>
          <div>
            <label htmlFor="severity" className={label}>{t("severity")}</label>
            <Select id="severity" name="severity" defaultValue={q.severity}>
              <option value="">{t("allSeverities")}</option>
              {(["critical", "high", "medium", "low"] as const).map((s) => (
                <option key={s} value={s}>{t(`sev.${s}`)} ({fmt.number(res.facets.severities[s] ?? 0)})</option>
              ))}
            </Select>
          </div>
          <div>
            <label htmlFor="level" className={label}>{t("level")}</label>
            <Select id="level" name="level" defaultValue={q.level}>
              <option value="">{t("allLevels")}</option>
              <option value="1">{t("levelN", { level: 1 })}</option>
              <option value="2">{t("levelN", { level: 2 })}</option>
            </Select>
          </div>
          <div>
            <label htmlFor="type" className={label}>{t("type")}</label>
            <Select id="type" name="type" defaultValue={q.type}>
              <option value="">{t("allTypes")}</option>
              <option value="automated">{t("automated")}</option>
              <option value="manual">{t("manual")}</option>
            </Select>
          </div>
          <div className="flex items-end gap-2 sm:col-span-2 lg:col-span-6">
            <Button type="submit">{t("apply")}</Button>
            {filtered && <Button asChild variant="outline"><Link href="/rules">{t("reset")}</Link></Button>}
          </div>
        </form>

        <p className="mt-6 flex flex-wrap items-baseline gap-x-2 text-sm text-muted-foreground" role="status" aria-live="polite">
          <strong className="text-base text-foreground">{t("results", { count: res.total })}</strong>
          {filtered && <span>{t("ofTotal", { total: fmt.number(res.catalogTotal) })}</span>}
          <span className="ms-auto">{t("pageOf", { page: res.page, pages: res.pages })}</span>
        </p>

        {res.catalogTotal === 0 ? (
          <p className="glass mt-4 p-8 text-center text-muted-foreground">{t("empty")}</p>
        ) : res.total === 0 ? (
          <div className="glass mt-4 p-10 text-center">
            <p className="font-medium">{t("noResults")}</p>
            <p className="mt-1 text-sm text-muted-foreground">{t("noResultsHint")}</p>
          </div>
        ) : (
          <div className="glass mt-4 overflow-hidden">
            <div className="overflow-x-auto">
              <table className="data-table min-w-[34rem]">
                <thead>
                  <tr>
                    <th scope="col">{t("cols.id")}</th>
                    <th scope="col">{t("cols.title")}</th>
                    <th scope="col" className="hidden md:table-cell">{t("cols.target")}</th>
                    <th scope="col">{t("cols.severity")}</th>
                    <th scope="col" className="hidden md:table-cell">{t("cols.level")}</th>
                    <th scope="col" className="hidden lg:table-cell">{t("cols.type")}</th>
                  </tr>
                </thead>
                <tbody>
                  {res.hits.map((h) => (
                    <tr key={h.id}>
                      <td className="font-mono text-xs"><Link href={`/rules/${encodeURIComponent(h.id)}`} className="link [overflow-wrap:anywhere]">{h.id}</Link></td>
                      <td lang="en">
                        <span className="[overflow-wrap:anywhere]">{h.title}</span>
                        <span className="mt-1 block font-mono text-xs text-muted-foreground md:hidden">{h.target} · {t("levelN", { level: h.level })}</span>
                      </td>
                      <td className="hidden md:table-cell">
                        <span className="inline-flex items-center gap-2 font-mono text-xs text-muted-foreground">
                          <PlatformIcon id={h.target} category={TARGETS[h.target]?.category} size={16} />
                          {h.target}
                        </span>
                      </td>
                      <td><SeverityBadge severity={h.severity} /></td>
                      <td className="hidden md:table-cell">{h.level}</td>
                      <td className="hidden lg:table-cell"><Badge variant={h.manual ? "medium" : "accent"}>{h.manual ? t("manual") : t("automated")}</Badge></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {res.pages > 1 && (
          <nav aria-label={t("pagination")} className="mt-6 flex flex-wrap items-center justify-center gap-2">
            {res.page > 1 ? (
              <Button asChild variant="outline" size="sm"><Link href={href(res.page - 1)} rel="prev"><ArrowLeft className="h-4 w-4 rtl:rotate-180" aria-hidden />{t("prev")}</Link></Button>
            ) : null}
            {pageNumbers[0] > 1 && (
              <>
                <Button asChild variant="ghost" size="sm"><Link href={href(1)}>1</Link></Button>
                {pageNumbers[0] > 2 && <span aria-hidden className="px-1 text-muted-foreground">…</span>}
              </>
            )}
            {pageNumbers.map((n) => (
              <Button key={n} asChild variant={n === res.page ? "default" : "ghost"} size="sm">
                <Link href={href(n)} aria-current={n === res.page ? "page" : undefined}>{fmt.number(n)}</Link>
              </Button>
            ))}
            {pageNumbers[pageNumbers.length - 1] < res.pages && (
              <>
                {pageNumbers[pageNumbers.length - 1] < res.pages - 1 && <span aria-hidden className="px-1 text-muted-foreground">…</span>}
                <Button asChild variant="ghost" size="sm"><Link href={href(res.pages)}>{fmt.number(res.pages)}</Link></Button>
              </>
            )}
            {res.page < res.pages ? (
              <Button asChild variant="outline" size="sm"><Link href={href(res.page + 1)} rel="next">{t("next")}<ArrowRight className="h-4 w-4 rtl:rotate-180" aria-hidden /></Link></Button>
            ) : null}
          </nav>
        )}

        <aside className="mt-10 rounded-lg border border-border bg-muted/40 p-4 text-xs leading-relaxed text-muted-foreground" aria-label="CIS">
          {t("attribution")}{" "}
          <Link href="/legal#cis" className="link underline">{t("attributionLink")}</Link>
        </aside>
      </div>
    </>
  );
}
