import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { CodeBlock } from "@/components/code-block";
import { PlatformIcon } from "@/components/platform-icon";
import { SeverityBadge } from "@/components/severity-badge";
import { Badge } from "@/components/ui/badge";
import { Callout } from "@/components/ui/callout";
import { Link } from "@/i18n/navigation";
import { getRule, type Rule } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { TARGETS } from "@/lib/targets";

// Rule pages are rendered on first request and then cached (ISR); nothing is prerendered for 6,000 rules x 16 locales.
export const revalidate = 86400;
export const dynamicParams = true;
export function generateStaticParams() {
  return [];
}

type P = { params: Promise<{ locale: string; id: string }> };

export async function generateMetadata({ params }: P): Promise<Metadata> {
  const { locale, id } = await params;
  const rule = getRule(decodeURIComponent(id));
  const t = await getTranslations({ locale, namespace: "meta" });
  if (!rule) return { title: t("pages.notFound"), robots: { index: false } };
  return pageMetadata({
    locale,
    path: `/rules/${encodeURIComponent(rule.id)}`,
    title: `${rule.id}: ${rule.title}`,
    description: rule.description.slice(0, 200),
    ogTitleEn: rule.title,
  });
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1 sm:grid-cols-[11rem_1fr] sm:gap-4">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-sm [overflow-wrap:anywhere]">{children}</dd>
    </div>
  );
}

/** Commands run in a shell; other values (manual steps, guidance) are prose. */
const CODE_TYPES = new Set(["command", "sql_exec", "sql_query", "registry", "file_regex", "config_regex", "cloud_check"]);

export default async function RulePage({ params }: P) {
  const { locale, id } = await params;
  setRequestLocale(locale);
  const rule: Rule | null = getRule(decodeURIComponent(id));
  if (!rule) notFound();
  const t = await getTranslations("rules");
  const d = (k: string, v?: Record<string, string>) => t(`detail.${k}`, v);
  const auditType = rule.audit.type;
  const manual = auditType === "manual";
  const auditTypeLabel = t.has(`auditTypes.${auditType}`) ? t(`auditTypes.${auditType}`) : auditType;
  const remType = rule.remediation.type ?? "";
  const remTypeLabel = remType && t.has(`remediationTypes.${remType}`) ? t(`remediationTypes.${remType}`) : remType;
  const auditText = rule.audit.command ?? rule.audit.check ?? "";
  const remCode = remType === "command" || remType === "sql_exec";

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "TechArticle",
    headline: `${rule.id}: ${rule.title}`,
    description: rule.description.slice(0, 300),
    inLanguage: "en",
    isPartOf: { "@type": "WebSite", name: "TMAegis" },
  };

  return (
    <article className="mx-auto max-w-4xl px-4 pb-8 pt-10 sm:px-6 sm:pt-14">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd).replace(/</g, "\\u003c") }} />
      <Link href={`/rules?target=${encodeURIComponent(rule.target)}`} className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-4 w-4 rtl:rotate-180" aria-hidden />{d("moreFor", { target: rule.target })}
      </Link>
      <div className="mt-6 flex items-center gap-3">
        <PlatformIcon id={rule.target} category={TARGETS[rule.target]?.category} size={44} tile active />
        <div className="min-w-0">
          <p className="text-sm font-medium" lang="en">{TARGETS[rule.target]?.name ?? rule.target}</p>
          <p className="font-mono text-sm text-primary [overflow-wrap:anywhere]">{rule.id}</p>
        </div>
      </div>
      <h1 lang="en" className="mt-4 text-balance text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">{rule.title}</h1>
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <SeverityBadge severity={rule.severity} />
        <Badge variant="default">{t("levelN", { level: rule.level })}</Badge>
        <Badge variant={manual ? "medium" : "accent"}>{manual ? t("manual") : t("automated")}</Badge>
        <Badge variant="default" className="font-mono">{rule.target}</Badge>
      </div>

      <div className="mt-8 space-y-10">
        <section aria-labelledby="r-desc">
          <h2 id="r-desc" className="mb-3 text-xl font-semibold">{d("description")}</h2>
          <p lang="en" className="whitespace-pre-line text-pretty text-muted-foreground [overflow-wrap:anywhere]">{rule.description}</p>
        </section>

        <section aria-labelledby="r-audit" className="space-y-3">
          <h2 id="r-audit" className="text-xl font-semibold">{d("audit")}</h2>
          <p className="text-sm text-muted-foreground"><span className="font-medium text-foreground">{d("auditType")}:</span> {auditTypeLabel}</p>
          {manual ? (
            <>
              <Callout>{d("manualNote")}</Callout>
              {auditText && <p className="whitespace-pre-line [overflow-wrap:anywhere]">{auditText}</p>}
            </>
          ) : (
            <>
              {auditText && (CODE_TYPES.has(auditType) ? <CodeBlock code={auditText} lang={auditType === "command" ? "shell" : auditType} /> : <p className="whitespace-pre-line [overflow-wrap:anywhere]">{auditText}</p>)}
              <dl className="space-y-2">
                {rule.audit.file_path && <Field label={d("filePath")}><code className="font-mono">{rule.audit.file_path}</code></Field>}
                {rule.audit.pattern && <Field label={d("pattern")}><code className="font-mono">{rule.audit.pattern}</code></Field>}
                {rule.audit.expected_output && <Field label={d("expected")}><code className="font-mono">{rule.audit.expected_output}</code></Field>}
              </dl>
            </>
          )}
        </section>

        <section aria-labelledby="r-fix" className="space-y-3">
          <h2 id="r-fix" className="text-xl font-semibold">{d("remediation")}</h2>
          {remType || rule.remediation.command ? (
            <>
              {remTypeLabel && <p className="text-sm text-muted-foreground"><span className="font-medium text-foreground">{d("remediationType")}:</span> {remTypeLabel}</p>}
              {rule.remediation.command && (remCode ? <CodeBlock code={rule.remediation.command} /> : <p className="whitespace-pre-line [overflow-wrap:anywhere]">{rule.remediation.command}</p>)}
              <dl className="space-y-2">
                {rule.remediation.file_path && <Field label={d("filePath")}><code className="font-mono">{rule.remediation.file_path}</code></Field>}
                {rule.remediation.replace_line && <Field label={d("replaceLine")}><code className="font-mono">{rule.remediation.replace_line}</code></Field>}
              </dl>
            </>
          ) : (
            <p className="text-muted-foreground">{d("noRemediation")}</p>
          )}
        </section>

        <section aria-labelledby="r-props">
          <h2 id="r-props" className="mb-3 text-xl font-semibold">{d("properties")}</h2>
          <dl className="glass space-y-3 p-5">
            <Field label={d("osFamily")}>{rule.os_family?.length ? rule.os_family.join(", ") : d("anyOs")}</Field>
            {rule.tags?.length ? <Field label={d("tags")}><span className="flex flex-wrap gap-1.5">{rule.tags.map((x) => <Badge key={x} variant="default">{x}</Badge>)}</span></Field> : null}
            {rule.reference && <Field label={d("reference")}>{rule.reference}</Field>}
          </dl>
        </section>

        {!manual && (
          <section aria-labelledby="r-try" className="space-y-3">
            <h2 id="r-try" className="text-xl font-semibold">{d("tryIt")}</h2>
            <p className="text-sm text-muted-foreground">{d("tryItNote")}</p>
            <CodeBlock code={`tmaegis scan --target ${rule.target}${rule.level === 2 ? " --level 2" : ""}`} />
          </section>
        )}
      </div>

      <aside className="mt-12 rounded-lg border border-border bg-muted/40 p-4 text-xs leading-relaxed text-muted-foreground">
        {t("attribution")} <Link href="/legal#cis" className="link underline">{t("attributionLink")}</Link>
      </aside>
    </article>
  );
}
