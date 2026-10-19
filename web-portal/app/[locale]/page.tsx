import type { Metadata } from "next";
import { ArrowRight, Boxes, Cloud, FileText, Lock, Plug, RotateCcw, ScanSearch, ShieldCheck, Wrench, Undo2, FileCode2, WifiOff, Eye, Camera, CheckCheck, Timer } from "lucide-react";
import { getFormatter, getTranslations, setRequestLocale } from "next-intl/server";
import { CodeBlock } from "@/components/code-block";
import { CliMock, GuiMock, ReportMock } from "@/components/mockups";
import { Section } from "@/components/page-header";
import { HeroIllustration } from "@/components/hero-illustration";
import { PlatformIcon } from "@/components/platform-icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Link } from "@/i18n/navigation";
import { LOCALES } from "@/lib/locales";
import { PLATFORM_GROUPS, WORKS_WITH } from "@/lib/platforms";
import { loadCoverage } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { GITHUB_URL } from "@/lib/site";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "", title: t("defaultTitle"), description: t("description"), ogTitleEn: "Open-source CIS hardening and compliance" });
}

const WHAT = [
  ["safe", RotateCcw],
  ["declarative", FileCode2],
  ["ai", Lock],
  ["cloud", Cloud],
  ["plugins", Plug],
  ["reports", FileText],
] as const;
const STEPS = [
  ["scan", ScanSearch],
  ["report", FileText],
  ["fix", Wrench],
  ["rollback", Undo2],
] as const;
const SAFETY = [
  ["snapshot", Camera],
  ["verify", CheckCheck],
  ["deadman", Timer],
  ["dryrun", Eye],
  ["offline", WifiOff],
  ["readonly", ShieldCheck],
] as const;

export default async function Home({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("home");
  const fmt = await getFormatter();
  const cov = loadCoverage();
  const stats: [string, string][] = [
    [fmt.number(cov.total), t("stats.rules")],
    [fmt.number(cov.targets), t("stats.targets")],
    [fmt.number(cov.auto), t("stats.automated")],
    [fmt.number(LOCALES.length), t("stats.languages")],
  ];

  return (
    <>
      {/* Hero */}
      <section className="mx-auto grid max-w-7xl items-center gap-10 overflow-x-clip px-4 pb-10 pt-12 sm:px-6 sm:pt-20 lg:grid-cols-[1.15fr_0.85fr]">
        <div className="min-w-0">
          <Badge variant="accent" className="mb-5"><span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden />{t("eyebrow")}</Badge>
          <h1 className="text-balance text-4xl font-semibold leading-[1.08] tracking-tight sm:text-6xl">{t("title")}</h1>
          <p className="mt-5 max-w-xl text-pretty text-lg text-muted-foreground">{t("lead")}</p>
          <p className="mt-3 max-w-xl text-sm text-muted-foreground">{t("betaNote")}</p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild size="lg"><Link href="/docs/quick-start">{t("ctaStart")}<ArrowRight className="h-4 w-4 rtl:rotate-180" aria-hidden /></Link></Button>
            <Button asChild size="lg" variant="outline"><Link href="/rules">{t("ctaRules")}</Link></Button>
            <Button asChild size="lg" variant="ghost"><Link href="/downloads">{t("ctaDownload")}</Link></Button>
          </div>
          <div className="mt-8 max-w-xl">
            <p className="label-caps mb-2">{t("installLabel")}</p>
            <CodeBlock code={"tmaegis scan --auto-detect\nsudo tmaegis apply --auto-detect --dry-run"} />
          </div>
        </div>
        <HeroIllustration label={t("heroArtAlt")} />
      </section>

      {/* Coverage numbers */}
      <section aria-label={t("stats.rules")} className="mx-auto max-w-7xl px-4 sm:px-6">
        <dl className="glass grid grid-cols-2 divide-border md:grid-cols-4 md:divide-x">
          {stats.map(([value, label], i) => (
            <div key={label} className={`px-5 py-6 text-center ${i > 1 ? "border-t border-border md:border-t-0" : ""} ${i % 2 === 1 ? "border-s border-border md:border-s-0" : ""}`}>
              <dd className="gradient-text text-3xl font-semibold tabular-nums sm:text-4xl">{value}</dd>
              <dt className="mt-1 text-sm text-muted-foreground">{label}</dt>
            </div>
          ))}
        </dl>
      </section>

      {/* Works with: a slow logo band (decorative; the names are listed in the grid further down) */}
      <section aria-labelledby="works-h" className="mx-auto max-w-7xl px-4 pt-14 sm:px-6">
        <h2 id="works-h" className="label-caps mb-5 text-center">{t("worksWith")}</h2>
        <div className="marquee" aria-hidden>
          <ul className="marquee-track py-1">
            {[0, 1].map((copy) =>
              WORKS_WITH.map(([name, icon]) => (
                <li key={`${copy}-${icon}`} data-copy={copy ? "" : undefined} className="group flex items-center gap-2 text-muted-foreground">
                  <PlatformIcon id={icon} size={22} />
                  <span lang="en" className="whitespace-nowrap text-sm font-medium">{name}</span>
                </li>
              )),
            )}
          </ul>
        </div>
      </section>

      {/* What you get */}
      <Section id="what" title={t("what.title")} intro={t("what.intro")} className="pt-20">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {WHAT.map(([key, Icon]) => (
            <Card key={key} className="glass-hover">
              <CardHeader>
                <span className="icon-tile mb-1"><Icon className="h-5 w-5" aria-hidden /></span>
                <CardTitle>{t(`what.items.${key}.title`)}</CardTitle>
                <CardDescription>{t(`what.items.${key}.text`)}</CardDescription>
              </CardHeader>
            </Card>
          ))}
        </div>
      </Section>

      {/* Platforms */}
      <Section id="platforms" title={t("platforms.title")} intro={t("platforms.intro")}>
        <div className="glass divide-y divide-border">
          {PLATFORM_GROUPS.map((g) => (
            <div key={g.id} className="grid gap-3 p-5 md:grid-cols-[14rem_1fr]">
              <h3 className="text-sm font-semibold text-muted-foreground">{t(`platforms.groups.${g.id}`)}</h3>
              <ul className="flex flex-wrap gap-2">
                {g.items.map(([n, icon]) => (
                  <li key={n} lang="en" className="group inline-flex items-center gap-2 rounded-full border border-border bg-muted/60 py-1 pe-3 ps-1.5 text-sm transition-colors hover:border-border-strong">
                    <PlatformIcon id={icon} size={16} />
                    {n}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <p className="mt-4 text-sm text-muted-foreground">
          <Link href="/platforms" className="link inline-flex items-center gap-1">
            {t("platforms.more", { count: Math.max(0, cov.targets - PLATFORM_GROUPS.reduce((n, g) => n + g.items.length, 0)) })}
            <ArrowRight className="h-4 w-4 rtl:rotate-180" aria-hidden />
          </Link>
        </p>
      </Section>

      {/* How it works */}
      <Section id="how" title={t("how.title")} intro={t("how.intro")}>
        <ol className="grid gap-4 md:grid-cols-4">
          {STEPS.map(([key, Icon], i) => (
            <li key={key} className="glass relative p-5">
              <div className="mb-3 flex items-center gap-3">
                <span className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-primary/40 bg-primary/10 text-primary"><Icon className="h-5 w-5" aria-hidden /></span>
                <span className="font-mono text-sm text-muted-foreground">0{i + 1}</span>
              </div>
              <h3 className="text-lg font-semibold">{t(`how.steps.${key}.title`)}</h3>
              <p className="mt-1.5 text-sm text-muted-foreground">{t(`how.steps.${key}.text`)}</p>
              {i < STEPS.length - 1 && <ArrowRight className="absolute -end-3.5 top-1/2 z-10 hidden h-5 w-5 -translate-y-1/2 rounded-full bg-background text-primary md:block rtl:rotate-180" aria-hidden />}
            </li>
          ))}
        </ol>
      </Section>

      {/* Screens */}
      <Section id="shots" title={t("shots.title")} intro={t("shots.intro")} className="cv-auto">
        <div className="grid gap-6 lg:grid-cols-3">
          <figure className="space-y-3"><CliMock /><figcaption className="text-sm font-medium text-muted-foreground">{t("shots.cli.caption")}</figcaption></figure>
          <figure className="space-y-3"><GuiMock /><figcaption className="text-sm font-medium text-muted-foreground">{t("shots.gui.caption")}</figcaption></figure>
          <figure className="space-y-3"><ReportMock /><figcaption className="text-sm font-medium text-muted-foreground">{t("shots.report.caption")}</figcaption></figure>
        </div>
      </Section>

      {/* Safety */}
      <Section id="safety" title={t("safety.title")} intro={t("safety.intro")} className="cv-auto">
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {SAFETY.map(([key, Icon]) => (
            <li key={key} className="glass flex gap-4 p-5">
              <Icon className="mt-0.5 h-6 w-6 shrink-0 text-accent-text" aria-hidden />
              <div>
                <h3 className="font-semibold">{t(`safety.items.${key}.title`)}</h3>
                <p className="mt-1 text-sm text-muted-foreground">{t(`safety.items.${key}.text`)}</p>
              </div>
            </li>
          ))}
        </ul>
      </Section>

      {/* Quick start */}
      <Section id="quick" title={t("quick.title")} intro={t("quick.intro")} className="cv-auto">
        <ol className="grid gap-5 md:grid-cols-2">
          {(
            [
              ["build", "git clone https://github.com/tmaegis/tmaegis.git\ncd tmaegis && make build"],
              ["scan", "./bin/tmaegis scan --auto-detect"],
              ["preview", "sudo ./bin/tmaegis apply --auto-detect --dry-run"],
              ["apply", "sudo ./bin/tmaegis apply --auto-detect\nsudo ./bin/tmaegis rollback"],
            ] as const
          ).map(([key, code], i) => (
            <li key={key} className="min-w-0 space-y-3">
              <div className="flex items-baseline gap-3">
                <span className="font-mono text-sm text-primary">0{i + 1}</span>
                <div>
                  <h3 className="font-semibold">{t(`quick.steps.${key}.title`)}</h3>
                  <p className="text-sm text-muted-foreground">{t(`quick.steps.${key}.text`)}</p>
                </div>
              </div>
              <CodeBlock code={code} />
            </li>
          ))}
        </ol>
        <p className="mt-6"><Link href="/docs/quick-start" className="link inline-flex items-center gap-1">{t("quick.more")}<ArrowRight className="h-4 w-4 rtl:rotate-180" aria-hidden /></Link></p>
      </Section>

      {/* CTA */}
      <section className="mx-auto max-w-7xl px-4 pt-8 sm:px-6">
        <div className="glass relative overflow-hidden p-8 text-center sm:p-12">
          <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(40rem_16rem_at_50%_0%,var(--glow-cyan),transparent_70%)]" aria-hidden />
          <Boxes className="mx-auto h-8 w-8 text-primary" aria-hidden />
          <h2 className="relative mt-4 text-balance text-3xl font-semibold tracking-tight">{t("cta.title")}</h2>
          <p className="relative mx-auto mt-3 max-w-xl text-muted-foreground">{t("cta.text")}</p>
          <div className="relative mt-7 flex flex-wrap justify-center gap-3">
            <Button asChild size="lg"><Link href="/docs/quick-start">{t("cta.primary")}</Link></Button>
            <Button asChild size="lg" variant="outline"><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">{t("cta.secondary")}</a></Button>
          </div>
        </div>
      </section>
    </>
  );
}
