import type { Metadata } from "next";
import { ChevronDown } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { PageHeader } from "@/components/page-header";
import { readGenerated, renderMarkdown } from "@/lib/markdown";
import { pageMetadata } from "@/lib/seo";
import { GITHUB_URL } from "@/lib/site";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/legal", title: t("pages.legal"), description: t("descriptions.legal"), ogTitleEn: "Legal" });
}

export default async function Legal({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("legal");
  const f = await getTranslations("footer");
  const c = await getTranslations("common");
  const trademark = renderMarkdown(readGenerated("TRADEMARK.md"));
  const notice = readGenerated("NOTICE.txt");
  const toc = ["license", "trademark", "notice", "privacy", "cis"] as const;
  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto grid max-w-7xl gap-10 px-4 sm:px-6 lg:grid-cols-[13rem_minmax(0,1fr)]">
        <nav aria-label={t("title")} className="lg:sticky lg:top-24 lg:self-start">
          <ul className="flex flex-wrap gap-2 lg:grid lg:gap-1">
            {toc.map((k) => (
              <li key={k}><a href={`#${k}`} className="block rounded-lg px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground">{t(`toc.${k}`)}</a></li>
            ))}
            <li><a href="#code-signing" lang="en" className="block rounded-lg px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground">Code signing policy</a></li>
          </ul>
        </nav>
        <div className="min-w-0 space-y-14">
          <section id="license" aria-labelledby="license-h" className="prose-tma">
            <h2 id="license-h" className="!mt-0">{t("license.title")}</h2>
            <p className="text-muted-foreground">{t("license.summary")}</p>
            <ul>{(["p1", "p2", "p3", "p4", "p5"] as const).map((p) => <li key={p}>{t(`license.${p}`)}</li>)}</ul>
            <p className="text-sm text-muted-foreground">{t("license.authoritative")}</p>
            <p><a href={`${GITHUB_URL}/blob/main/LICENSE`} target="_blank" rel="noopener noreferrer" hrefLang="en">{t("license.full")}</a></p>
          </section>

          <section id="trademark" aria-labelledby="trademark-h">
            <h2 id="trademark-h" className="text-2xl font-semibold">{t("trademark.title")}</h2>
            <div className="prose-tma mt-3">
              <h3>{t("trademark.summaryTitle")}</h3>
              <p className="text-muted-foreground">{t("trademark.summaryNote")}</p>
              <ul>{(["s1", "s2", "s3", "s4", "s5"] as const).map((k) => <li key={k}>{t(`trademark.${k}`)}</li>)}</ul>
            </div>
            <p className="mt-4 text-sm text-muted-foreground">{f("brands")}</p>
            <p className="mt-2 text-sm text-muted-foreground">{t("trademark.intro")}</p>
            <details className="group mt-5">
              <summary className="link inline-flex cursor-pointer list-none items-center gap-1.5 text-sm font-medium [&::-webkit-details-marker]:hidden">
                <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden />
                {c("showDetails")}
              </summary>
              <div lang="en" className="prose-tma glass mt-3 p-6" dangerouslySetInnerHTML={{ __html: trademark }} />
            </details>
          </section>

          <section id="notice" aria-labelledby="notice-h">
            <h2 id="notice-h" className="text-2xl font-semibold">{t("notice.title")}</h2>
            <div className="prose-tma mt-3">
              <h3>{t("notice.summaryTitle")}</h3>
              <ul>{(["s1", "s2", "s3", "s4"] as const).map((k) => <li key={k}>{t(`notice.${k}`)}</li>)}</ul>
            </div>
            <p className="mt-4 text-sm text-muted-foreground">{t("notice.intro")}</p>
            <details className="group mt-5">
              <summary className="link inline-flex cursor-pointer list-none items-center gap-1.5 text-sm font-medium [&::-webkit-details-marker]:hidden">
                <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden />
                {c("showDetails")}
              </summary>
              <pre lang="en" className="glass cmd-scroll mt-3 whitespace-pre-wrap p-6 text-xs leading-relaxed [overflow-wrap:anywhere]" tabIndex={0}>{notice}</pre>
            </details>
          </section>

          <section id="code-signing" aria-labelledby="code-signing-h" lang="en" className="prose-tma">
            <h2 id="code-signing-h" className="!mt-0">Code signing policy</h2>
            <p>
              Free code signing for the Windows builds of TMAegis is provided by <a href="https://signpath.io" target="_blank" rel="noopener noreferrer">SignPath.io</a>,
              certificate by <a href="https://signpath.org" target="_blank" rel="noopener noreferrer">SignPath Foundation</a>.
            </p>
            <ul>
              <li>Only binaries built from this project&apos;s source by its GitHub Actions workflows are signed: <code>tmaegis.exe</code> and <code>tmaegis-desktop.exe</code> inside the Windows .zip archives.</li>
              <li>Authors, reviewers and approvers: Tommy Huynh and Misha Milanovic. Every release signing request is approved manually, and everyone with repository or signing access uses multi-factor authentication.</li>
              <li>Checksums (<code>checksums.txt</code>) are regenerated after signing.</li>
              <li>Privacy: TMAegis does not send information to other networked systems unless the user asks for it.</li>
            </ul>
            <p><a href={`${GITHUB_URL}/blob/main/docs/CODE_SIGNING_POLICY.md`} target="_blank" rel="noopener noreferrer">Read the full policy</a></p>
          </section>

          <section id="privacy" aria-labelledby="privacy-h" className="prose-tma">
            <h2 id="privacy-h" className="!mt-0">{t("privacy.title")}</h2>
            <ul>{(["p1", "p2", "p3", "p4", "p5", "p6"] as const).map((p) => <li key={p}>{t(`privacy.${p}`)}</li>)}</ul>
          </section>

          <section id="cis" aria-labelledby="cis-h" className="prose-tma">
            <h2 id="cis-h" className="!mt-0">{t("cis.title")}</h2>
            <p>{t("cis.text")}</p>
          </section>
        </div>
      </div>
    </>
  );
}
