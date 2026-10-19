import type { Metadata } from "next";
import { Check, ChevronDown } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Link } from "@/i18n/navigation";
import { pageMetadata } from "@/lib/seo";
import { GITHUB_URL } from "@/lib/site";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/pricing", title: t("pages.pricing"), description: t("descriptions.pricing"), ogTitleEn: "Pricing and licensing" });
}

export default async function Pricing({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("pricing");
  const plans = [
    { key: "community", cta: <Button asChild variant="outline" size="lg"><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">{t("community.cta")}</a></Button> },
    { key: "commercial", cta: <Button asChild size="lg"><Link href="/contact">{t("commercial.cta")}</Link></Button> },
  ] as const;
  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto grid max-w-7xl gap-6 px-4 sm:px-6 md:grid-cols-2">
        {plans.map(({ key, cta }) => (
          <section key={key} aria-labelledby={`p-${key}`} className={`glass flex flex-col p-6 sm:p-7 ${key === "commercial" ? "ring-1 ring-primary/40" : ""}`}>
            <h2 id={`p-${key}`} className="text-2xl font-semibold">{t(`${key}.title`)}</h2>
            <p className="mt-1 text-muted-foreground">{t(`${key}.tagline`)}</p>
            <ul className="mt-6 flex-1 space-y-3">
              {(["p1", "p2", "p3", "p4"] as const).map((p) => (
                <li key={p} className="flex gap-3 text-sm"><Check className="mt-0.5 h-4 w-4 shrink-0 text-accent-text" aria-hidden /><span>{t(`${key}.${p}`)}</span></li>
              ))}
            </ul>
            <div className="mt-8">{cta}</div>
          </section>
        ))}
      </div>
      <p className="mx-auto mt-6 max-w-7xl px-4 text-sm text-muted-foreground sm:px-6">{t("noPrices")}</p>

      <div className="mx-auto mt-12 grid max-w-7xl gap-6 px-4 sm:px-6 md:grid-cols-2">
        <section aria-labelledby="p-partners" className="glass p-7">
          <h2 id="p-partners" className="text-xl font-semibold">{t("partners.title")}</h2>
          <p className="mt-2 text-muted-foreground">{t("partners.text")}</p>
          <Button asChild variant="outline" className="mt-5"><Link href="/contact">{t("partners.cta")}</Link></Button>
        </section>
        <section aria-labelledby="p-faq" className="space-y-4">
          <h2 id="p-faq" className="text-xl font-semibold">{t("faq.title")}</h2>
          {(["1", "2", "3"] as const).map((n) => (
            <details key={n} className="glass group">
              <summary className="flex cursor-pointer items-center gap-3 rounded-[var(--radius)] p-4 font-medium hover:bg-muted/40">
                <span className="min-w-0 flex-1">{t(`faq.q${n}`)}</span>
                <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden />
              </summary>
              <p className="px-4 pb-4 text-sm text-muted-foreground">{t(`faq.a${n}`)}</p>
            </details>
          ))}
        </section>
      </div>
    </>
  );
}
