import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { DocsBody } from "@/components/docs-body";
import { NavLinks } from "@/components/nav-links";
import { Callout } from "@/components/ui/callout";
import { isTranslated } from "@/i18n/messages";
import { routing } from "@/i18n/routing";
import { Link } from "@/i18n/navigation";
import { DOCS } from "@/lib/docs-content";
import { pageMetadata } from "@/lib/seo";
import { DOC_SLUGS, GITHUB_URL, type DocSlug } from "@/lib/site";

export const dynamicParams = false;
export function generateStaticParams() {
  return routing.locales.flatMap((locale) => DOC_SLUGS.map((slug) => ({ locale, slug })));
}

type P = { params: Promise<{ locale: string; slug: string }> };
const isDoc = (s: string): s is DocSlug => (DOC_SLUGS as readonly string[]).includes(s);

export async function generateMetadata({ params }: P): Promise<Metadata> {
  const { locale, slug } = await params;
  if (!isDoc(slug)) return {};
  const t = await getTranslations({ locale, namespace: "docs.nav" });
  return pageMetadata({ locale, path: `/docs/${slug}`, title: t(`${slug}.title`), description: t(`${slug}.summary`), ogTitleEn: `Docs: ${slug}` });
}

export default async function DocPage({ params }: P) {
  const { locale, slug } = await params;
  if (!isDoc(slug)) notFound();
  setRequestLocale(locale);
  const t = await getTranslations("docs");
  const body = await getTranslations("docsBody");
  const english = !(await isTranslated(locale, `docsBody.${slug}`));
  const blocks = DOCS[slug];
  const toc = blocks.flatMap((b) => (b.t === "h2" ? [{ id: b.id, text: String(body.raw(`${slug}.${b.k}`)) }] : []));
  const i = DOC_SLUGS.indexOf(slug);
  const prev = i > 0 ? DOC_SLUGS[i - 1] : null;
  const next = i < DOC_SLUGS.length - 1 ? DOC_SLUGS[i + 1] : null;
  const nav = DOC_SLUGS.map((s) => ({ href: `/docs/${s}`, label: t(`nav.${s}.title`) }));

  return (
    <div className="mx-auto grid max-w-7xl gap-10 px-4 pb-8 pt-10 sm:px-6 lg:grid-cols-[14rem_minmax(0,1fr)_13rem]">
      <nav aria-label={t("pages")} className="lg:sticky lg:top-24 lg:self-start">
        <NavLinks items={nav} className="flex flex-wrap gap-1 lg:grid" />
      </nav>
      <article className="min-w-0">
        <h1 className="text-balance text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">{t(`nav.${slug}.title`)}</h1>
        <p className="mt-2 text-muted-foreground">{t(`nav.${slug}.summary`)}</p>
        {english && locale !== "en" && locale !== "en-gb" && <Callout className="mt-4">{t("englishOnly")}</Callout>}
        <div className="mt-8"><DocsBody slug={slug} blocks={blocks} english={english} /></div>
        <div className="mt-12 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-6 text-sm">
          {prev ? <Link href={`/docs/${prev}`} className="link inline-flex items-center gap-1.5"><ArrowLeft className="h-4 w-4 rtl:rotate-180" aria-hidden />{t("previous")}: {t(`nav.${prev}.title`)}</Link> : <span />}
          {next ? <Link href={`/docs/${next}`} className="link inline-flex items-center gap-1.5">{t("next")}: {t(`nav.${next}.title`)}<ArrowRight className="h-4 w-4 rtl:rotate-180" aria-hidden /></Link> : <span />}
        </div>
        <p className="mt-6 text-sm"><a href={`${GITHUB_URL}/blob/main/web-portal/messages/en-US.json`} target="_blank" rel="noopener noreferrer" className="text-muted-foreground hover:text-foreground">{t("editOnGitHub")}</a></p>
      </article>
      <aside className="hidden lg:block">
        {toc.length > 0 && (
          <nav aria-label={t("onThisPage")} className="sticky top-24">
            <p className="label-caps mb-2">{t("onThisPage")}</p>
            <ul className="space-y-1.5 border-s border-border ps-3 text-sm">
              {toc.map((h) => (
                <li key={h.id}><a href={`#${h.id}`} lang={english ? "en" : undefined} className="text-muted-foreground hover:text-foreground">{h.text}</a></li>
              ))}
            </ul>
          </nav>
        )}
      </aside>
    </div>
  );
}
