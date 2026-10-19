import type { Metadata } from "next";
import { Check, Cloud, FileCode2, FileText, Languages, Lock, Network, Plug, RotateCcw, ScanSearch } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { pageMetadata } from "@/lib/seo";

const SECTIONS = [
  ["scan", ScanSearch],
  ["safety", RotateCcw],
  ["rules", FileCode2],
  ["cloud", Cloud],
  ["network", Network],
  ["ai", Lock],
  ["reports", FileText],
  ["plugins", Plug],
  ["languages", Languages],
] as const;

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/features", title: t("pages.features"), description: t("descriptions.features"), ogTitleEn: "Features" });
}

export default async function Features({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("features");
  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto grid max-w-7xl gap-5 px-4 pb-8 sm:px-6 md:grid-cols-2">
        {SECTIONS.map(([key, Icon]) => (
          <section key={key} aria-labelledby={`f-${key}`} className="glass glass-hover p-6">
            <div className="flex items-center gap-3">
              <span className="icon-tile"><Icon className="h-5 w-5" aria-hidden /></span>
              <h2 id={`f-${key}`} className="text-xl font-semibold">{t(`sections.${key}.title`)}</h2>
            </div>
            <p className="mt-3 text-muted-foreground">{t(`sections.${key}.text`)}</p>
            <ul className="mt-4 space-y-2 text-sm">
              {(["p1", "p2", "p3"] as const).map((p) => (
                <li key={p} className="flex gap-2.5">
                  <Check className="mt-0.5 h-4 w-4 shrink-0 text-accent-text" aria-hidden />
                  <span>{t(`sections.${key}.${p}`)}</span>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </>
  );
}
