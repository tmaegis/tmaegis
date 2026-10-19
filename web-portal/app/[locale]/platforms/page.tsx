import type { Metadata } from "next";
import { getFormatter, getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { PlatformBrowser, type PlatformCategory } from "@/components/platforms/platform-browser";
import { Link } from "@/i18n/navigation";
import { loadCoverage } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { groupTargets, scanCommand } from "@/lib/targets";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/platforms", title: t("pages.platforms"), description: t("descriptions.platforms"), ogTitleEn: "Platforms" });
}

export default async function Platforms({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("platforms");
  const h = await getTranslations("downloads.hardens");
  const fmt = await getFormatter();
  const cov = loadCoverage();
  const { categories } = groupTargets(cov.rows);
  const data: PlatformCategory[] = categories.map((c) => ({
    id: c.category,
    groups: c.groups.map((g) => ({
      id: g.id,
      rows: g.rows.map((r) => ({ id: r.id, name: r.name, total: r.count, auto: r.auto, manual: r.manual, device: (r.device ?? 0) > 0, command: scanCommand(r) })),
    })),
  }));
  const flags = [
    ["scan", "tmaegis scan --target <target>"],
    ["device", "tmaegis scan --target <target> --device-config <file>"],
    ["manual", "tmaegis scan --target <target> --include-manual"],
  ];

  return (
    <>
      <PageHeader title={t("title")} intro={t("intro", { rules: cov.total, targets: cov.targets })} />
      <div className="mx-auto max-w-7xl px-4 pb-16 sm:px-6">
        <div className="mb-6 grid gap-3 sm:grid-cols-3">
          {flags.map(([k, c]) => (
            <div key={k} className="glass p-4">
              <p className="text-sm font-medium">{h(`flags.${k}`)}</p>
              <code className="mt-1.5 block break-all font-mono text-xs text-muted-foreground" dir="ltr">{c}</code>
            </div>
          ))}
        </div>
        <PlatformBrowser categories={data} />
        <p className="mt-8 max-w-3xl text-sm text-muted-foreground">
          {t("coverageNote", { rules: fmt.number(cov.total) })}{" "}
          <Link href="/coverage" className="link">{h("coverageLink")}</Link>
        </p>
      </div>
    </>
  );
}
