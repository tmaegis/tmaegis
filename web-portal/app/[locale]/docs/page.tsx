import type { Metadata } from "next";
import { ArrowRight, BookOpen, Download, FileText, MonitorSmartphone, Play, Puzzle, SquareTerminal, type LucideIcon } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { isTranslated } from "@/i18n/messages";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Link } from "@/i18n/navigation";
import { pageMetadata } from "@/lib/seo";
import { DOC_SLUGS } from "@/lib/site";

const DOC_ICON: Record<string, LucideIcon> = { install: Download, "quick-start": Play, cli: SquareTerminal, "rules-format": BookOpen, plugins: Puzzle, reports: FileText, gui: MonitorSmartphone };

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/docs", title: t("pages.docs"), description: t("descriptions.docs"), ogTitleEn: "Documentation" });
}

export default async function DocsIndex({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("docs");
  const allTranslated = (await Promise.all(DOC_SLUGS.map((s) => isTranslated(locale, `docsBody.${s}`)))).every(Boolean);
  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        {!allTranslated && <p className="mb-6 text-sm text-muted-foreground">{t("englishOnly")}</p>}
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {DOC_SLUGS.map((s) => (
            <li key={s}>
              <Link href={`/docs/${s}`} className="group block h-full rounded-[var(--radius)]">
                <Card className="glass-hover h-full">
                  <CardHeader>
                    {(() => {
                      const Icon = DOC_ICON[s] ?? BookOpen;
                      return <span className="icon-tile mb-1"><Icon className="h-5 w-5" aria-hidden /></span>;
                    })()}
                    <CardTitle as="h2" className="flex items-center justify-between gap-2">{t(`nav.${s}.title`)}<ArrowRight className="h-4 w-4 shrink-0 text-primary rtl:rotate-180" aria-hidden /></CardTitle>
                    <CardDescription>{t(`nav.${s}.summary`)}</CardDescription>
                  </CardHeader>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </>
  );
}
