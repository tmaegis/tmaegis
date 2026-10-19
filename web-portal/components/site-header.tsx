import { getTranslations } from "next-intl/server";
import { Menu } from "lucide-react";
import { Link } from "@/i18n/navigation";
import { GITHUB_URL } from "@/lib/site";
import { Badge } from "@/components/ui/badge";
import { LogoMark } from "./logo";
import { GitHubIcon } from "./github-icon";
import { LanguageSwitcher } from "./language-switcher";
import { NavLinks, type NavItem } from "./nav-links";
import { ThemeToggle } from "./theme-toggle";

export async function SiteHeader() {
  const t = await getTranslations("nav");
  const c = await getTranslations("common");
  const items: NavItem[] = [
    { href: "/features", label: t("features") },
    { href: "/rules", label: t("rules") },
    { href: "/platforms", label: t("platforms") },
    { href: "/coverage", label: t("coverage") },
    { href: "/docs", label: t("docs") },
    { href: "/downloads", label: t("downloads") },
    { href: "/pricing", label: t("pricing") },
  ];
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/70 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:px-6">
        <Link href="/" className="flex items-center gap-2.5 rounded-lg font-semibold tracking-tight" aria-label={t("homeLink")}>
          <LogoMark size={32} />
          <span className="text-lg">TMAegis</span>
        </Link>
        <Badge variant="medium" className="-ms-1 px-2 py-0 text-[0.6875rem] uppercase tracking-wide">{c("beta")}</Badge>
        <nav aria-label={t("primary")} className="ms-2 hidden lg:block xl:ms-4">
          <NavLinks items={items} className="flex items-center gap-0.5" linkClass="whitespace-nowrap px-2 xl:px-3" />
        </nav>
        <div className="ms-auto flex items-center gap-2">
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("github")}
            className="hidden h-10 items-center gap-2 whitespace-nowrap rounded-lg border border-border bg-card px-3 text-sm font-medium hover:border-primary sm:inline-flex"
          >
            <GitHubIcon className="h-4 w-4" />
            <span className="lg:hidden xl:inline">{t("github")}</span>
          </a>
          <ThemeToggle />
          <LanguageSwitcher />
          <details className="group relative lg:hidden">
            <summary className="inline-flex h-10 w-10 cursor-pointer items-center justify-center rounded-lg border border-border bg-card" aria-label={t("openMenu")}>
              <Menu className="h-5 w-5" aria-hidden />
            </summary>
            <nav aria-label={t("primary")} className="glass absolute end-0 z-50 mt-2 w-64 max-w-[calc(100vw-2rem)] p-2 shadow-2xl" style={{ background: "var(--card-solid)" }}>
              <NavLinks items={[{ href: "/", label: t("home") }, ...items, { href: "/contact", label: t("contact") }]} className="grid gap-0.5" linkClass="py-2.5 text-base" />
            </nav>
          </details>
        </div>
      </div>
    </header>
  );
}
