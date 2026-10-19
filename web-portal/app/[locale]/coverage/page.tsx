import type { Metadata } from "next";
import { ChevronDown } from "lucide-react";
import { getFormatter, getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { CategoryIcon, PlatformIcon } from "@/components/platform-icon";
import { Link } from "@/i18n/navigation";
import { loadCoverage } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { groupTargets, TARGETS } from "@/lib/targets";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/coverage", title: t("pages.coverage"), description: t("descriptions.coverage"), ogTitleEn: "Coverage" });
}

export default async function Coverage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("coverage");
  const h = await getTranslations("downloads.hardens");
  const fmt = await getFormatter();
  const cov = loadCoverage();
  const catLabel = (c: string) => (t.has(`categories.${c}`) ? t(`categories.${c}`) : c);
  const groups = cov.categories
    .map((c) => ({ id: c, label: catLabel(c), rows: cov.rows.filter((r) => r.category === c) }))
    .filter((g) => g.rows.length)
    .sort((a, b) => a.label.localeCompare(b.label, locale));
  const totals: [string, number][] = [
    [t("totals.rules"), cov.total],
    [t("totals.targets"), cov.targets],
    [t("totals.automated"), cov.auto],
    [t("totals.manual"), cov.manual],
  ];
  // Rules per category (the categories of /platforms), largest first: one bar each, value and share labelled.
  const cats = groupTargets(cov.rows).categories.slice().sort((a, b) => b.rules - a.rules);
  const maxRules = Math.max(1, ...cats.map((c) => c.rules));

  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        <dl className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {totals.map(([label, v]) => (
            <div key={label} className="glass p-4">
              <dd className="gradient-text text-3xl font-semibold tabular-nums">{fmt.number(v)}</dd>
              <dt className="text-sm text-muted-foreground">{label}</dt>
            </div>
          ))}
        </dl>

        <section aria-labelledby="cov-cats" className="glass mt-8 p-5 sm:p-6">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 id="cov-cats" className="text-lg font-semibold">{t("visual.title")}</h2>
            <p className="text-sm text-muted-foreground">{t("visual.intro")}</p>
          </div>
          <ul className="mt-5 grid gap-x-10 gap-y-3 md:grid-cols-2">
            {cats.map((c) => (
              <li key={c.category} className="group">
                <Link href={`/platforms#${c.category}`} className="grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 rounded-lg p-1.5 hover:bg-muted/40">
                  <CategoryIcon id={c.category} size={32} tile />
                  <span className="min-w-0">
                    <span className="flex flex-wrap items-baseline justify-between gap-x-2 text-sm">
                      <span className="min-w-0 font-medium [overflow-wrap:anywhere]">{h(`categories.${c.category}`)}</span>
                      <span className="text-xs text-muted-foreground">{t("visual.targets", { count: c.targets })}</span>
                    </span>
                    <span className="mt-1.5 block h-2 overflow-hidden rounded-full bg-muted" aria-hidden>
                      <span className="block h-full rounded-full bg-primary" style={{ width: `${Math.max(2, (c.rules / maxRules) * 100)}%` }} />
                    </span>
                  </span>
                  <span className="w-14 text-end text-sm font-semibold tabular-nums">{fmt.number(c.rules)}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>

        <div className="mt-8 space-y-3">
          {groups.map((g, i) => (
            <details key={g.id} open={i === 0} className="glass group overflow-hidden [&_summary::-webkit-details-marker]:hidden">
              <summary className="flex cursor-pointer list-none items-center gap-3 px-4 py-3 hover:bg-muted/30 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary">
                <span className="flex-1 text-sm font-semibold uppercase tracking-wider text-accent-text">{g.label}</span>
                <span className="text-sm tabular-nums text-muted-foreground">{fmt.number(g.rows.reduce((n, r) => n + r.count, 0))}</span>
                <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden />
              </summary>
              <div className="overflow-x-auto border-t border-border">
                <table className="w-full min-w-[44rem] border-collapse text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/60 text-xs uppercase tracking-wider text-muted-foreground">
                      <th scope="col" className="px-4 py-3 text-start font-semibold">{t("cols.platform")}</th>
                      <th scope="col" className="px-4 py-3 text-start font-semibold">{t("cols.benchmark")}</th>
                      <th scope="col" className="px-4 py-3 text-end font-semibold">{t("cols.rules")}</th>
                      <th scope="col" className="hidden px-4 py-3 text-end font-semibold sm:table-cell">{t("cols.automated")}</th>
                      <th scope="col" className="hidden px-4 py-3 text-end font-semibold sm:table-cell">{t("cols.manual")}</th>
                      <th scope="col" className="hidden px-4 py-3 text-start font-semibold lg:table-cell">{t("cols.how")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {g.rows.map((r) => (
                      <tr key={r.id} className="align-top hover:bg-muted/40">
                        <th scope="row" className="px-4 py-3 text-start font-mono text-xs font-medium">
                          <Link href={`/rules?target=${encodeURIComponent(r.id)}`} className="text-primary hover:underline" aria-label={t("viewRulesFor", { target: r.id })}>{r.id}</Link>
                        </th>
                        <td className="px-4 py-3 text-muted-foreground" lang="en">
                          {r.benchmarks.length ? r.benchmarks.map((b) => <span key={b} className="block">{b}</span>) : t("noBenchmark")}
                        </td>
                        <td className="px-4 py-3 text-end tabular-nums font-medium">{fmt.number(r.count)}</td>
                        <td className="hidden px-4 py-3 text-end tabular-nums sm:table-cell">{fmt.number(r.auto)}</td>
                        <td className="hidden px-4 py-3 text-end tabular-nums sm:table-cell">{fmt.number(r.manual)}</td>
                        <td className="hidden px-4 py-3 text-muted-foreground lg:table-cell" lang="en">{r.how || "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </details>
          ))}
        </div>
        <p className="mt-6 max-w-3xl text-sm text-muted-foreground">{t("note")}</p>
      </div>
    </>
  );
}
