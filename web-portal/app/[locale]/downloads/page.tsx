import type { Metadata } from "next";
import { ChevronDown, ClipboardList, ExternalLink, FileText, Layers, Network, Package, Server, ShieldCheck, Terminal, Unplug } from "lucide-react";
import { getFormatter, getTranslations, setRequestLocale } from "next-intl/server";
import { CodeBlock } from "@/components/code-block";
import { DevicesBrowser } from "@/components/downloads/devices-browser";
import { FlowDiagram } from "@/components/downloads/flow-diagram";
import { Notes } from "@/components/downloads/notes";
import { OsTabs } from "@/components/downloads/os-tabs";
import { Recommended } from "@/components/downloads/recommended";
import { PageHeader, Section } from "@/components/page-header";
import { CategoryIcon, PlatformIcon } from "@/components/platform-icon";
import { Badge } from "@/components/ui/badge";
import { Callout } from "@/components/ui/callout";
import { Button } from "@/components/ui/button";
import { Link } from "@/i18n/navigation";
import { buildGuideRows } from "@/lib/device-guides";
import { SOURCE_BUILD } from "@/lib/install-commands";
import { parseNotes } from "@/lib/release-notes";
import { checksumsAsset } from "@/lib/release-assets";
import { loadReleaseData } from "@/lib/releases";
import { loadCoverage } from "@/lib/rules-data";
import { pageMetadata } from "@/lib/seo";
import { GITHUB_URL } from "@/lib/site";

// Incremental static regeneration: the release list is refreshed from GitHub at most every 10 minutes.
export const revalidate = 600; // keep equal to REVALIDATE_SECONDS in lib/releases.ts (Next needs a literal)

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/downloads", title: t("pages.downloads"), description: t("descriptions.downloads"), ogTitleEn: "Downloads" });
}

const SECTIONS = ["recommended", "os", "flow", "devices", "components", "hardens", "verify", "requirements", "notes", "license"] as const;
const REQUIREMENTS = ["windows", "macos", "linux", "unix"] as const;
const SUMMARY = [
  ["one", Layers],
  ["network", Network],
  ["remote", Unplug],
  ["manual", ClipboardList],
] as const;
const COMPONENTS = [
  ["cli", Terminal, "tmaegis scan --auto-detect\ntmaegis gui"],
  ["server", Server, "export TMAEGIS_SERVER_TOKEN=$(openssl rand -hex 24)\ndocker compose up -d\ntmaegis scan --auto-detect --push-url http://localhost:8080"],
  ["reports", FileText, "tmaegis scan --auto-detect --format html --output report.html\ntmaegis scan --auto-detect --format xlsx --output report.xlsx\ntmaegis scan --auto-detect --format pdf --output report.pdf"],
  ["rules", Package, "tmaegis rules export -o rules.json\ntmaegis scan --rules-dir ./rules --target linux"],
] as const;

export default async function Downloads({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("downloads");
  const c = await getTranslations("common");
  const fmt = await getFormatter();
  const data = await loadReleaseData();
  const rel = data.latest;
  const cov = loadCoverage();
  const archives = rel ? rel.assets.filter((a) => a.os && a.arch && a.component === "tmaegis") : [];
  const checksums = rel ? checksumsAsset(rel.assets)?.url ?? null : null;
  const notes = rel ? parseNotes(rel.body) : [];
  const date = (iso: string | null) => (iso && !Number.isNaN(Date.parse(iso)) ? fmt.dateTime(new Date(iso), { dateStyle: "long" }) : null);
  const published = rel ? date(rel.publishedAt) : null;
  const exampleFile = rel?.assets.find((a) => a.kind === "archive" && a.os === "linux" && a.arch === "x64")?.name ?? "tmaegis_0.1.0_linux_amd64.tar.gz";
  const exampleWin = rel?.assets.find((a) => a.kind === "archive" && a.os === "windows" && a.arch === "x64")?.name ?? "tmaegis_0.1.0_windows_amd64.zip";
  const exampleMac = rel?.assets.find((a) => a.kind === "archive" && a.os === "macos" && a.arch === "arm64")?.name ?? "tmaegis_0.1.0_darwin_arm64.tar.gz";
  const sections = SECTIONS.filter((s) => (rel ? true : s !== "os"));
  const guides = buildGuideRows(cov.rows).rows;

  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")}>
        <div className="mt-5 flex flex-wrap items-center gap-3">
          {rel ? (
            <>
              <Badge variant="accent">{t("version", { version: rel.tag })}</Badge>
              <Badge variant="medium">{c("beta")}</Badge>
              {published && <span className="text-sm text-muted-foreground">{t("published", { date: published })}</span>}
              <Badge variant="default"><ShieldCheck className="h-3.5 w-3.5" aria-hidden />{data.source === "github" ? t("status.live") : t("status.lastKnown")}</Badge>
            </>
          ) : (
            <Badge variant="medium">{t("comingSoon.badge")}</Badge>
          )}
        </div>
        <Callout tone="warning" className="mt-4 max-w-3xl">{t.rich("beta.callout", { code: (chunks) => <code className="rounded bg-muted px-1 font-mono text-[0.9em]">{chunks}</code> })}</Callout>
        {data.source === "fallback" && <Callout tone="warning" className="mt-4 max-w-3xl">{t("status.fallback")}</Callout>}
      </PageHeader>

      <nav aria-label={t("nav.label")} className="sticky top-16 z-30 border-y border-border bg-background/85 backdrop-blur-xl">
        <ul className="mx-auto flex max-w-7xl gap-1 overflow-x-auto px-4 py-2 sm:px-6">
          {sections.map((s) => (
            <li key={s}>
              <a href={`#${s}`} className="block whitespace-nowrap rounded-lg px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground">{t(`nav.${s}`)}</a>
            </li>
          ))}
        </ul>
      </nav>

      {rel ? (
        <>
          <Section id="recommended" title={t("recommended.title")} intro={t("recommended.intro")} className="scroll-mt-28 pt-10">
            <noscript><p className="mb-4 rounded-lg border border-border bg-muted/40 p-3 text-sm">{t("recommended.noscript")}</p></noscript>
            <Recommended assets={archives} checksumsUrl={checksums} />
          </Section>
          <Section id="os" title={t("os.title")} intro={t("os.intro")} className="scroll-mt-28">
            <OsTabs assets={archives} version={rel.version} checksumsUrl={checksums} image={data.image} githubUrl={GITHUB_URL} />
          </Section>
        </>
      ) : (
        <Section id="recommended" title={t("comingSoon.title")} intro={t("comingSoon.text")} className="scroll-mt-28 pt-10">
          <div className="grid gap-6 lg:grid-cols-2">
            <div className="min-w-0 space-y-3">
              <h3 className="font-semibold">{t("os.source.title")}</h3>
              <p className="text-sm text-muted-foreground">{t("os.source.intro")}</p>
              <CodeBlock code={SOURCE_BUILD} />
            </div>
            <div className="glass space-y-3 p-5">
              <p className="text-sm text-muted-foreground">{t("comingSoon.follow")}</p>
              <div className="flex flex-wrap gap-3">
                <Button asChild><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">{t("comingSoon.repo")}<ExternalLink className="h-4 w-4" aria-hidden /></a></Button>
                <Button asChild variant="outline"><a href={`${GITHUB_URL}/releases`} target="_blank" rel="noopener noreferrer">{t("allReleases")}</a></Button>
              </div>
            </div>
          </div>
        </Section>
      )}

      <Section id="flow" title={t("flow.title")} intro={t("flow.intro")} className="scroll-mt-28">
        <FlowDiagram />
      </Section>

      <Section id="devices" title={t("devices.title")} intro={t("devices.intro")} className="scroll-mt-28">
        <div className="mb-8">
          <h3 className="mb-3 text-lg font-semibold">{t("devices.summary.title")}</h3>
          <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {SUMMARY.map(([key, Icon]) => (
              <li key={key} className="glass flex gap-3 p-4">
                <Icon className="mt-0.5 h-5 w-5 shrink-0 text-accent-text" aria-hidden />
                <div className="min-w-0">
                  <p className="font-semibold">{t(`devices.summary.${key}.title`)}</p>
                  <p className="mt-1 text-sm text-muted-foreground">{key === "one" ? t("devices.summary.one.text", { targets: cov.targets }) : t(`devices.summary.${key}.text`)}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <DevicesBrowser rows={guides} githubUrl={GITHUB_URL} />
      </Section>

      <Section id="components" title={t("components.title")} intro={t("components.intro")} className="scroll-mt-28">
        <div className="grid gap-4 md:grid-cols-2">
          {COMPONENTS.map(([key, Icon, code]) => (
            <article key={key} className="glass flex min-w-0 flex-col gap-3 p-5">
              <div className="flex items-center gap-3">
                <span className="icon-tile"><Icon className="h-5 w-5" aria-hidden /></span>
                <h3 className="text-lg font-semibold">{t(`components.${key}.title`)}</h3>
              </div>
              <p className="text-sm text-muted-foreground">{t(`components.${key}.text`)}</p>
              <CodeBlock code={code} className="mt-auto" />
              {key === "cli" && <p className="text-xs text-muted-foreground">{t("components.cli.windowsGui")}</p>}
              {key === "server" && <p className="text-xs text-muted-foreground">{t("components.server.note")}</p>}
              {key === "reports" && <p className="text-xs"><Link href="/docs/reports" className="link">{t("components.reports.more")}</Link></p>}
              {key === "rules" && <p className="text-xs"><Link href="/rules" className="link">{t("components.rules.more")}</Link> · <Link href="/docs/rules-format" className="link">{t("components.rules.format")}</Link></p>}
            </article>
          ))}
        </div>
      </Section>

      <Section id="hardens" title={t("hardens.title")} intro={t("hardens.teaser", { rules: cov.total, targets: cov.targets })} className="scroll-mt-28">
        <ul className="flex flex-wrap gap-2">
          {(["os", "server", "database", "containers", "network", "cloud", "ai"] as const).map((c) => (
            <li key={c}>
              <Link href={`/platforms#${c}`} className="chip">
                <CategoryIcon id={c} size={16} />
                {t(`hardens.categories.${c}`)}
              </Link>
            </li>
          ))}
        </ul>
        <p className="mt-5"><Button asChild><Link href="/platforms">{t("hardens.browse")}</Link></Button></p>
      </Section>

      <Section id="verify" title={t("verify.title")} intro={t("verify.intro")} className="scroll-mt-28">
        <details className="group">
          <summary className="link inline-flex cursor-pointer list-none items-center gap-1.5 text-sm font-medium [&::-webkit-details-marker]:hidden">
            <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden />
            {c("showDetails")}
          </summary>
        <div className="mt-4 grid gap-6 lg:grid-cols-2">
          <div className="min-w-0 space-y-3">
            <h3 className="flex items-center gap-2 font-semibold"><PlatformIcon id="windows" size={18} />Windows (PowerShell)</h3>
            <CodeBlock lang="powershell" code={`Get-FileHash .\\${exampleWin} -Algorithm SHA256\nSelect-String -Path .\\checksums.txt -Pattern ${exampleWin}`} />
            <h3 className="flex items-center gap-2 pt-2 font-semibold"><PlatformIcon id="macos" size={18} />macOS</h3>
            <CodeBlock code={`shasum -a 256 ${exampleMac}\ngrep ${exampleMac} checksums.txt`} />
            <h3 className="flex items-center gap-2 pt-2 font-semibold"><PlatformIcon id="linux" size={18} />Linux</h3>
            <CodeBlock code={`sha256sum -c checksums.txt --ignore-missing\nsha256sum ${exampleFile}`} />
            <h3 className="flex items-center gap-2 pt-2 font-semibold"><PlatformIcon id="unix" size={18} />{t("verify.unixTitle")}</h3>
            <CodeBlock code={`# FreeBSD\nsha256 ${exampleFile.replace("linux_amd64", "freebsd_amd64")}\n# Solaris\ndigest -a sha256 ${exampleFile.replace("linux_amd64", "solaris_amd64")}\n# AIX (or any system with OpenSSL)\nopenssl dgst -sha256 ${exampleFile.replace("linux_amd64", "aix_ppc64")}\n\ngrep <file name> checksums.txt`} />
          </div>
          <ul className="min-w-0 space-y-3">
            {(["source", "match", "unsigned", "report"] as const).map((k) => (
              <li key={k} className="glass flex gap-3 p-4">
                <ShieldCheck className="mt-0.5 h-5 w-5 shrink-0 text-accent-text" aria-hidden />
                <div>
                  <h3 className="font-semibold">{t(`verify.items.${k}.title`)}</h3>
                  <p className="mt-1 text-sm text-muted-foreground">{t(`verify.items.${k}.text`)}</p>
                </div>
              </li>
            ))}
            <li className="flex flex-wrap gap-3">
              <Button asChild variant="outline" size="sm"><a href={`${GITHUB_URL}/blob/main/SECURITY.md`} target="_blank" rel="noopener noreferrer">{t("verify.securityPolicy")}<ExternalLink className="h-3.5 w-3.5" aria-hidden /></a></Button>
              {checksums && <Button asChild variant="ghost" size="sm"><a href={checksums} rel="noopener noreferrer">{t("verify.checksumsFile")}</a></Button>}
            </li>
          </ul>
        </div>
        </details>
      </Section>

      <Section id="requirements" title={t("requirements.title")} intro={t("requirements.intro")} className="scroll-mt-28">
        <div className="glass overflow-x-auto">
          <table className="data-table min-w-[34rem]">
            <thead>
              <tr>
                <th scope="col">{t("requirements.cols.os")}</th>
                <th scope="col">{t("requirements.cols.cpu")}</th>
                <th scope="col">{t("requirements.cols.needs")}</th>
              </tr>
            </thead>
            <tbody>
              {REQUIREMENTS.map((k) => (
                <tr key={k} className="group">
                  <th scope="row" className="font-semibold">
                    <span className="flex items-center gap-2.5"><PlatformIcon id={k} size={28} tile />{t(`requirements.${k}.name`)}</span>
                  </th>
                  <td className="text-muted-foreground">{t(`requirements.${k}.cpu`)}</td>
                  <td className="text-muted-foreground">{t(`requirements.${k}.needs`)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-4 text-sm text-muted-foreground">{t("requirements.note")}</p>
      </Section>

      <Section id="notes" title={t("notes.title")} className="scroll-mt-28">
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
          <div className="glass min-w-0 p-6">
            {rel ? (
              <>
                <div className="flex flex-wrap items-center gap-3">
                  <h3 className="text-lg font-semibold">{rel.name}</h3>
                  {published && <span className="text-sm text-muted-foreground">{published}</span>}
                </div>
                {notes.length > 0 ? <div className="mt-3"><Notes blocks={notes} /></div> : <p className="mt-3 text-sm text-muted-foreground">{t("notes.empty")}</p>}
                <Button asChild variant="outline" size="sm" className="mt-5"><a href={rel.url} target="_blank" rel="noopener noreferrer">{t("notes.onGitHub")}<ExternalLink className="h-3.5 w-3.5" aria-hidden /></a></Button>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">{t("notes.none")}</p>
            )}
          </div>
          <div className="glass min-w-0 p-6">
            <h3 className="text-lg font-semibold">{t("notes.previous")}</h3>
            {data.previous.length > 0 ? (
              <ul className="mt-3 divide-y divide-border text-sm">
                {data.previous.map((p) => (
                  <li key={p.tag} className="flex flex-wrap items-center justify-between gap-2 py-2">
                    <a href={p.url} target="_blank" rel="noopener noreferrer" className="link font-mono">{p.tag}</a>
                    <span className="text-muted-foreground">{date(p.publishedAt) ?? ""}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">{t("notes.noPrevious")}</p>
            )}
            <Button asChild variant="ghost" size="sm" className="mt-4"><a href={`${GITHUB_URL}/releases`} target="_blank" rel="noopener noreferrer">{t("allReleases")}<ExternalLink className="h-3.5 w-3.5" aria-hidden /></a></Button>
          </div>
        </div>
      </Section>

      <Section id="license" title={t("license.title")} className="scroll-mt-28">
        <div className="glass grid gap-4 p-6 md:grid-cols-2 [&>*]:min-w-0">
          <p className="text-sm">{t("license.free")}</p>
          <p className="text-sm text-muted-foreground">{t("license.trademark")}</p>
          <p lang="en" className="text-sm text-muted-foreground md:col-span-2">
            Code signing policy: free code signing for the Windows builds is provided by{" "}
            <a href="https://signpath.io" target="_blank" rel="noopener noreferrer" className="link">SignPath.io</a>, certificate by{" "}
            <a href="https://signpath.org" target="_blank" rel="noopener noreferrer" className="link">SignPath Foundation</a>.{" "}
            <Link href="/legal#code-signing" className="link">Read the policy</Link>. To uninstall, delete the folder you unpacked; nothing else is installed.
          </p>
          <div className="flex flex-wrap gap-3 md:col-span-2">
            <Button asChild variant="outline" size="sm"><Link href="/legal#license">{t("license.licenseLink")}</Link></Button>
            <Button asChild variant="outline" size="sm"><Link href="/legal#trademark">{t("license.trademarkLink")}</Link></Button>
          </div>
        </div>
      </Section>
    </>
  );
}
