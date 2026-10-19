import { getTranslations } from "next-intl/server";
import { Link } from "@/i18n/navigation";
import { Badge } from "@/components/ui/badge";
import { LogoMark } from "./logo";
import { GITHUB_URL } from "@/lib/site";

export async function SiteFooter() {
  const t = await getTranslations("footer");
  const n = await getTranslations("nav");
  const c = await getTranslations("common");
  const col = "grid gap-2 text-sm text-muted-foreground [&_a]:rounded-sm [&_a:hover]:text-foreground";
  const head = "label-caps mb-3 text-foreground";
  return (
    <footer className="mt-24 border-t border-border bg-background/60">
      <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6">
        <div className="grid gap-10 md:grid-cols-[1.4fr_repeat(3,1fr)]">
          <div className="space-y-3">
            <div className="flex items-center gap-2.5 font-semibold">
              <LogoMark size={28} />
              TMAegis<span aria-hidden>™</span>
              <Badge variant="medium" className="px-2 py-0 text-[0.6875rem] uppercase tracking-wide">{c("beta")}</Badge>
            </div>
            <p className="max-w-xs text-sm text-muted-foreground">{t("tagline")}</p>
          </div>
          <nav aria-label={t("product")}>
            <h2 className={head}>{t("product")}</h2>
            <ul className={col}>
              <li><Link href="/features">{n("features")}</Link></li>
              <li><Link href="/rules">{n("rules")}</Link></li>
              <li><Link href="/platforms">{n("platforms")}</Link></li>
              <li><Link href="/coverage">{n("coverage")}</Link></li>
              <li><Link href="/downloads">{n("downloads")}</Link></li>
            </ul>
          </nav>
          <nav aria-label={t("resources")}>
            <h2 className={head}>{t("resources")}</h2>
            <ul className={col}>
              <li><Link href="/docs">{n("docs")}</Link></li>
              <li><Link href="/pricing">{n("pricing")}</Link></li>
              <li><Link href="/contact">{n("contact")}</Link></li>
              <li><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">{t("sourceCode")}</a></li>
            </ul>
          </nav>
          <nav aria-label={t("company")}>
            <h2 className={head}>{n("legal")}</h2>
            <ul className={col}>
              <li><Link href="/legal#license">{t("license")}</Link></li>
              <li><Link href="/legal#trademark">{t("trademark")}</Link></li>
              <li><Link href="/legal#notice">{t("notice")}</Link></li>
              <li><Link href="/legal#privacy">{t("privacy")}</Link></li>
            </ul>
          </nav>
        </div>
        <div className="mt-10 space-y-2 border-t border-border pt-6 text-xs leading-relaxed text-muted-foreground">
          <p>{t("copyright")}</p>
          <p>{t("marks")}</p>
          <p>{t("brands")}</p>
          <p>{t("noCookies")}</p>
        </div>
      </div>
    </footer>
  );
}
