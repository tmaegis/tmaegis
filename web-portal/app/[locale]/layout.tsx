import type { Metadata, Viewport } from "next";
import { notFound } from "next/navigation";
import { hasLocale, NextIntlClientProvider } from "next-intl";
import { getTranslations, setRequestLocale } from "next-intl/server";
import type { ReactNode } from "react";
import { CjkFont } from "@/components/cjk-font";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { loadMessages } from "@/i18n/messages";
import { routing } from "@/i18n/routing";
import { fontClass } from "@/lib/fonts";
import { localeInfo } from "@/lib/locales";
import { pageMetadata } from "@/lib/seo";
import { alternates, siteUrl } from "@/lib/site";

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#0b0f19" },
    { media: "(prefers-color-scheme: light)", color: "#f5f9fd" },
  ],
};

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) return {};
  const t = await getTranslations({ locale, namespace: "meta" });
  const page = await pageMetadata({ locale, path: "", title: t("defaultTitle"), description: t("description"), ogTitleEn: "Open-source CIS hardening and compliance" });
  return {
    ...page,
    metadataBase: new URL(siteUrl()),
    title: { default: t("defaultTitle"), template: "%s · TMAegis" },
    alternates: { languages: alternates("") },
    applicationName: "TMAegis",
    authors: [{ name: "Tommy Huynh" }, { name: "Misha Milanovic" }],
  };
}

// Applies the saved theme before first paint (localStorage only, no cookie).
const THEME_INIT = `try{var t=localStorage.getItem("tma-theme");if(t==="dark"||t==="light")document.documentElement.dataset.theme=t}catch(e){}`;

// Client components only need these namespaces; the rest of the catalog never ships to the browser.
const CLIENT_NAMESPACES = ["common", "nav", "lang", "contact", "rules", "downloads", "platforms"];

export default async function LocaleLayout({ children, params }: { children: ReactNode; params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) notFound();
  setRequestLocale(locale);
  const info = localeInfo(locale);
  const t = await getTranslations({ locale, namespace: "nav" });
  const all = await loadMessages(locale);
  const messages = Object.fromEntries(CLIENT_NAMESPACES.map((k) => [k, all[k]]));

  return (
    <html lang={info.tag} dir={info.dir} data-script={info.script} className={fontClass} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT }} />
      </head>
      <body>
        <CjkFont script={info.script} />
        <div className="aurora" aria-hidden />
        <div className="hexgrid" aria-hidden />
        <NextIntlClientProvider locale={locale} messages={messages}>
          <a href="#main" className="skip-link">{t("skip")}</a>
          <SiteHeader />
          <main id="main" tabIndex={-1} className="outline-none">{children}</main>
          <SiteFooter />
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
