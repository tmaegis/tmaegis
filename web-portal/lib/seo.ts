import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { alternates } from "./site";
import { isCjk, localeInfo } from "./locales";

interface PageMeta {
  locale: string;
  /** locale-less path, e.g. "/rules" */
  path: string;
  title: string;
  description: string;
  /** Plain-English title for the OG image when the locale's script is not covered by the OG font. */
  ogTitleEn?: string;
  noindex?: boolean;
}

/** Builds localized metadata: canonical, hreflang alternates, Open Graph + Twitter with a per-page image. */
export async function pageMetadata({ locale, path, title, description, ogTitleEn, noindex }: PageMeta): Promise<Metadata> {
  const info = localeInfo(locale);
  const t = await getTranslations({ locale, namespace: "meta" });
  const ogTitle = isCjk(locale) && ogTitleEn ? ogTitleEn : title;
  const og = `/og?${new URLSearchParams({ title: ogTitle.slice(0, 90), kicker: isCjk(locale) ? "Open-source CIS hardening" : t("ogKicker") })}`;
  const url = `/${locale}${path}`;
  return {
    title,
    description,
    alternates: { canonical: url, languages: alternates(path) },
    robots: noindex ? { index: false, follow: true } : undefined,
    openGraph: {
      type: "website",
      siteName: "TMAegis",
      title,
      description,
      url,
      locale: info.tag.replace("-", "_"),
      images: [{ url: og, width: 1200, height: 630, alt: title }],
    },
    twitter: { card: "summary_large_image", title, description, images: [og] },
  };
}
