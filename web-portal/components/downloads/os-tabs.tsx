"use client";

import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { ExternalLink } from "lucide-react";
import { useTranslations } from "next-intl";
import { CodeBlock } from "@/components/code-block";
import { CopyButton } from "@/components/copy-button";
import { PlatformIcon } from "@/components/platform-icon";
import { Badge } from "@/components/ui/badge";
import { Callout } from "@/components/ui/callout";
import { linuxFamily } from "@/lib/detect-platform";
import { archiveCommand, crossBuildCommand, DOCKER_BUILD, dockerCiCommand, dockerDeviceCommand, dockerPullCommand, packageCommand, removeCommand, SOURCE_BUILD } from "@/lib/install-commands";
import { EXPERIMENTAL, findLinuxPackage, findPackage, type Arch, type ReleaseAsset } from "@/lib/release-assets";
import type { ContainerImageInfo } from "@/lib/releases";
import { AssetCard, MissingCard } from "./asset-card";
import { archLabel, OS_ICON, OS_NAME } from "./format";
import { Segment } from "./segment";

export const OS_TABS = ["windows", "macos", "linux", "unix", "docker", "source"] as const;
export type OsTab = (typeof OS_TABS)[number];

const ARCHES: Record<"windows" | "macos" | "linux", Arch[]> = { windows: ["x64", "arm64"], macos: ["arm64", "x64"], linux: ["x64", "arm64"] };
const LINUX_KINDS = ["archive", "deb", "rpm"] as const;
type LinuxKind = (typeof LINUX_KINDS)[number];

/** Install methods that do not exist yet: names only, never a command. */
const PLANNED: Partial<Record<OsTab, string[]>> = { windows: ["winget", "Scoop", ".msi installer"], macos: ["Homebrew", ".pkg installer"] };

function tabFromHash(hash: string): OsTab | null {
  const id = hash.replace(/^#(os-)?/, "");
  return (OS_TABS as readonly string[]).includes(id) ? (id as OsTab) : null;
}

function Steps({ children }: { children: ReactNode }) {
  return <ol className="list-decimal space-y-2 ps-5 text-sm marker:text-muted-foreground">{children}</ol>;
}

function Note({ children, tone = "muted" }: { children: ReactNode; tone?: "muted" | "warning" }) {
  return <Callout tone={tone === "warning" ? "warning" : "info"}>{children}</Callout>;
}

/** Icon for each tab (lib/platform-icons.ts). */
const TAB_ICON: Record<OsTab, string> = { windows: "windows", macos: "macos", linux: "linux", unix: "unix", docker: "docker", source: "source" };

/** Windows / macOS / Linux / Unix and other / Docker / source tabs. Deep-linkable: #os-windows ... #os-unix, #os-docker, #os-source. */
export function OsTabs({ assets, version, checksumsUrl, image, githubUrl }: { assets: ReleaseAsset[]; version: string; checksumsUrl: string | null; image: ContainerImageInfo | null; githubUrl: string }) {
  const t = useTranslations("downloads");
  const [tab, setTab] = useState<OsTab>("windows");
  const [linuxKind, setLinuxKind] = useState<LinuxKind>("archive");
  const [linuxArch, setLinuxArch] = useState<Arch>("x64");
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});
  const checksums = checksumsUrl;

  useEffect(() => {
    const sync = () => {
      const next = tabFromHash(window.location.hash);
      if (next) setTab(next);
    };
    // Preselect the package format when the browser names the distribution (rare; the archive is the default).
    const preselect = () => {
      const ua = navigator.userAgent ?? "";
      const fam = linuxFamily(ua);
      const fmt = fam === "debian" ? "deb" : fam ? "rpm" : null;
      if (fmt && (findLinuxPackage(assets, fmt, "x64") || findLinuxPackage(assets, fmt, "arm64"))) setLinuxKind(fmt);
      if (/linux/i.test(ua) && /aarch64|arm64/i.test(ua)) setLinuxArch("arm64");
    };
    sync();
    preselect();
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, [assets]);

  const select = useCallback((next: OsTab, focus = false) => {
    setTab(next);
    try {
      history.replaceState(null, "", `#os-${next}`);
    } catch {
      /* some embedded browsers block history changes */
    }
    if (focus) refs.current[next]?.focus();
  }, []);

  const onKey = (e: KeyboardEvent<HTMLButtonElement>, i: number) => {
    const last = OS_TABS.length - 1;
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const fwd = rtl ? "ArrowLeft" : "ArrowRight";
    const back = rtl ? "ArrowRight" : "ArrowLeft";
    const move = e.key === fwd ? (i === last ? 0 : i + 1) : e.key === back ? (i === 0 ? last : i - 1) : e.key === "Home" ? 0 : e.key === "End" ? last : -1;
    if (move >= 0) {
      e.preventDefault();
      select(OS_TABS[move], true);
    }
  };

  const optionsHeading = (count: number) => (
    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
      <h3 className="text-lg font-semibold">{t("os.optionsTitle")}</h3>
      <span className="text-sm text-muted-foreground">{t("os.optionsCount", { count })}</span>
    </div>
  );

  const planned = (id: OsTab) =>
    PLANNED[id] ? (
      <aside aria-label={t("os.planned.title")} className="rounded-lg border border-dashed border-border p-3 text-sm">
        <p className="font-medium">{t("os.planned.title")}</p>
        <ul className="mt-2 flex flex-wrap gap-2">
          {PLANNED[id]!.map((p) => (
            <li key={p}><Badge variant="default" className="font-normal" lang="en">{p}</Badge></li>
          ))}
        </ul>
        <p className="mt-2 text-xs text-muted-foreground">{t("os.planned.text")}</p>
      </aside>
    ) : null;

  /** Archive cards for Windows and macOS, one per processor, each with the command for that exact file. */
  const archiveCards = (os: "windows" | "macos") => {
    const extras = assets.filter((a) => a.os === os && (a.kind === "installer" || a.kind === "package"));
    return (
      <>
        <div className="grid gap-4 sm:grid-cols-2">
          {ARCHES[os].map((arch) => {
            const a = findPackage(assets, os, arch);
            return a && a.kind === "archive" ? (
              <AssetCard
                key={arch}
                asset={a}
                os={os}
                arch={arch}
                checksumsUrl={checksums}
                title={t("os.archiveTitle", { format: a.format === "zip" ? "ZIP" : a.format })}
                command={archiveCommand(os, a.name)}
                commandLang={os === "windows" ? "powershell" : "shell"}
                headingLevel={4}
              />
            ) : (
              <MissingCard key={arch} title={`${OS_NAME[os]} · ${archLabel(os, arch, t)}`} detail={t("os.notInRelease", { version })} />
            );
          })}
        </div>
        {extras.length > 0 && (
          <div className="space-y-3">
            <h4 className="text-sm font-semibold">{t("os.alsoInRelease")}</h4>
            <div className="grid gap-4 sm:grid-cols-2">
              {extras.map((x) => <AssetCard key={x.name} asset={x} os={os} arch={x.arch ?? "x64"} checksumsUrl={checksums} headingLevel={4} />)}
            </div>
          </div>
        )}
      </>
    );
  };

  const linuxAsset = linuxKind === "archive" ? (() => { const a = findPackage(assets, "linux", linuxArch); return a?.kind === "archive" ? a : null; })() : findLinuxPackage(assets, linuxKind, linuxArch);
  const linuxCount = (["archive", "deb", "rpm"] as const).filter((k) => ARCHES.linux.some((arch) => (k === "archive" ? findPackage(assets, "linux", arch)?.kind === "archive" : findLinuxPackage(assets, k, arch)))).length;
  const kindLabel: Record<LinuxKind, string> = { archive: t("os.pkg.archive"), deb: t("os.pkg.deb"), rpm: t("os.pkg.rpm") };
  const kindTitle: Record<LinuxKind, string> = { archive: t("os.pkg.archiveTitle"), deb: t("os.pkg.debTitle"), rpm: t("os.pkg.rpmTitle") };
  const kindFormat: Record<LinuxKind, string> = { archive: ".tar.gz", deb: ".deb", rpm: ".rpm" };

  const imageAvailable = image && image.status !== "missing";

  const panels: Record<OsTab, ReactNode> = {
    windows: (
      <div className="space-y-5">
        {optionsHeading(ARCHES.windows.filter((a) => findPackage(assets, "windows", a)?.kind === "archive").length ? 1 : 0)}
        {archiveCards("windows")}
        <Steps>
          <li>{t("os.windows.s1")}</li>
          <li>{t("os.windows.s2")}</li>
          <li>{t("os.windows.s3")}</li>
        </Steps>
        <p className="text-xs text-muted-foreground">{t("os.fileNote")}</p>
        <Callout title={t("os.windows.smartscreenTitle").replace(/[:：]\s*$/, "")}>{t("os.windows.smartscreen")}</Callout>
        {planned("windows")}
      </div>
    ),
    macos: (
      <div className="space-y-5">
        {optionsHeading(ARCHES.macos.filter((a) => findPackage(assets, "macos", a)?.kind === "archive").length ? 1 : 0)}
        {archiveCards("macos")}
        <Steps>
          <li>{t("os.macos.s1")}</li>
          <li>{t("os.macos.s2")}</li>
        </Steps>
        <p className="text-xs text-muted-foreground">{t("os.fileNote")}</p>
        <Callout title={t("os.macos.gatekeeperTitle").replace(/[:：]\s*$/, "")}>{t("os.macos.gatekeeper")}</Callout>
        {planned("macos")}
      </div>
    ),
    linux: (
      <div className="space-y-5">
        {optionsHeading(linuxCount)}
        <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
          <Segment label={t("os.pkg.label")} value={linuxKind} options={LINUX_KINDS.map((k) => ({ id: k, text: kindLabel[k] }))} onChange={setLinuxKind} />
          <Segment label={t("recommended.archLabel")} value={linuxArch} options={ARCHES.linux.map((a) => ({ id: a, text: archLabel("linux", a, t) }))} onChange={setLinuxArch} />
        </div>
        <div className="max-w-3xl" aria-live="polite">
          {linuxAsset ? (
            <AssetCard
              key={linuxAsset.name}
              asset={linuxAsset}
              os="linux"
              arch={linuxArch}
              checksumsUrl={checksums}
              title={kindTitle[linuxKind]}
              command={linuxKind === "archive" ? archiveCommand("linux", linuxAsset.name) : packageCommand(linuxKind, linuxAsset.name)}
              headingLevel={4}
            />
          ) : (
            <MissingCard title={`${kindTitle[linuxKind]} · ${archLabel("linux", linuxArch, t)}`} detail={t("os.pkg.notInRelease", { format: kindFormat[linuxKind], arch: archLabel("linux", linuxArch, t), version })} />
          )}
        </div>
        {linuxKind === "archive" ? (
          <Steps>
            <li>{t("os.linux.s1")}</li>
            <li>{t("os.linux.s2")}</li>
          </Steps>
        ) : (
          <div className="space-y-3">
            <Steps>
              <li>{t("os.linux.s1")}</li>
              <li>{t("os.pkg.installs")}</li>
            </Steps>
            <div className="max-w-3xl"><Note tone="warning">{t("os.pkg.unsigned")}</Note></div>
            {linuxAsset && (
              <div className="max-w-3xl">
                <p className="mb-1.5 text-sm font-medium">{t("os.pkg.remove")}</p>
                <CodeBlock code={removeCommand(linuxKind)} />
              </div>
            )}
          </div>
        )}
        <div className="max-w-3xl"><Note>{t("os.pkg.which")}</Note></div>
        <p className="text-xs text-muted-foreground">{t("os.fileNote")}</p>
      </div>
    ),
    unix: (
      <div className="space-y-5">
        {optionsHeading(EXPERIMENTAL.filter((e) => findPackage(assets, e.os, e.arch)?.kind === "archive").length)}
        <p className="max-w-3xl text-sm text-muted-foreground">{t("os.unix.intro")}</p>
        <div className="max-w-3xl">
          <Callout tone="experimental" title={t("os.unix.experimentalTitle")}>
            {t("os.unix.experimentalText")}{" "}
            <a href={`${githubUrl}/issues/new`} target="_blank" rel="noopener noreferrer" className="link inline-flex items-center gap-1">
              {t("os.unix.report")}
              <ExternalLink className="h-3.5 w-3.5" aria-hidden />
            </a>
          </Callout>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {EXPERIMENTAL.map(({ os, arch }) => {
            const a = findPackage(assets, os, arch);
            const name = `${OS_NAME[os]} · ${archLabel(os, arch, t)}`;
            return a && a.kind === "archive" ? (
              <AssetCard key={`${os}-${arch}`} asset={a} os={os} arch={arch} checksumsUrl={checksums} title={OS_NAME[os]} command={archiveCommand(os, a.name)} headingLevel={4} />
            ) : (
              <MissingCard key={`${os}-${arch}`} title={name} detail={t("os.unix.missing", { version })} icon={<PlatformIcon id={arch === "s390x" ? "s390x" : arch === "riscv64" ? "riscv" : OS_ICON[os]} size={18} />}>
                <CodeBlock code={crossBuildCommand(os, arch)} />
              </MissingCard>
            );
          })}
        </div>
        <Steps>
          <li>{t("os.unix.s1")}</li>
          <li>{t("os.unix.s2")}</li>
          <li>{t("os.unix.s3")}</li>
        </Steps>
        <p className="text-xs text-muted-foreground">{t("os.fileNote")}</p>
      </div>
    ),
    docker: (
      <div className="space-y-5">
        {optionsHeading(imageAvailable ? 1 : 0)}
        {image && (
          <article className="glass flex min-w-0 max-w-3xl flex-col gap-3 p-4" aria-label={t("os.docker.imageTitle")}>
            <div className="flex flex-wrap items-center gap-2">
              <PlatformIcon id="docker" size={32} tile active />
              <h4 className="text-base font-semibold">{t("os.docker.imageTitle")}</h4>
              <Badge variant="default">{t("os.docker.multiArch")}</Badge>
              <Badge variant={image.status === "published" ? "accent" : image.status === "missing" ? "medium" : "default"}>{t(`os.docker.status.${image.status}`)}</Badge>
            </div>
            <div className="flex min-w-0 items-center gap-2 rounded-lg border border-border bg-muted/50 p-2.5">
              <code className="min-w-0 flex-1 break-all font-mono text-xs" dir="ltr">{image.ref}</code>
              {imageAvailable && <CopyButton text={image.ref} label={image.ref} compact tone="themed" />}
            </div>
            {image.status === "missing" ? (
              <>
                <p className="text-sm text-muted-foreground">{t("os.docker.missing", { version: `v${version}` })}</p>
                <CodeBlock code={DOCKER_BUILD} />
              </>
            ) : (
              <>
                {image.status === "unknown" && <Note>{t("os.docker.unknown")}</Note>}
                <div>
                  <p className="label-caps mb-1.5">{t("os.commandLabel")}</p>
                  <CodeBlock code={dockerPullCommand(image.ref)} />
                </div>
                <p className="text-sm text-muted-foreground">{t("os.docker.inside")}</p>
              </>
            )}
          </article>
        )}
        {imageAvailable && image && (
          <div className="grid gap-4 lg:grid-cols-2">
            <div className="min-w-0 space-y-2">
              <h4 className="font-semibold">{t("os.docker.deviceTitle")}</h4>
              <p className="text-sm text-muted-foreground">{t("os.docker.deviceText")}</p>
              <CodeBlock code={dockerDeviceCommand(image.ref)} />
            </div>
            <div className="min-w-0 space-y-2">
              <h4 className="font-semibold">{t("os.docker.ciTitle")}</h4>
              <p className="text-sm text-muted-foreground">{t("os.docker.ciText")}</p>
              <CodeBlock code={dockerCiCommand(image.ref)} />
            </div>
          </div>
        )}
        <Note>{t("os.docker.minimal")}</Note>
        <Callout tone="warning" title={t("os.docker.hostTitle")}>{t("os.docker.hostText")}</Callout>
      </div>
    ),
    source: (
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">{t("os.source.intro")}</p>
        <CodeBlock code={SOURCE_BUILD} />
        <p className="text-sm text-muted-foreground">{t("os.source.cross")}</p>
      </div>
    ),
  };

  return (
    <div>
      <div role="tablist" aria-label={t("os.tablist")} className="track overflow-x-auto">
        {OS_TABS.map((id, i) => (
          <button
            key={id}
            ref={(el) => {
              refs.current[id] = el;
            }}
            id={`os-${id}`}
            role="tab"
            type="button"
            aria-selected={tab === id}
            aria-controls={`os-panel-${id}`}
            tabIndex={tab === id ? 0 : -1}
            onClick={() => select(id)}
            onKeyDown={(e) => onKey(e, i)}
            className="tab scroll-mt-32 shrink-0"
          >
            <PlatformIcon id={TAB_ICON[id]} size={18} />
            {id === "docker" ? "Docker" : id === "source" ? t("os.sourceTab") : id === "unix" ? t("os.unixTab") : OS_NAME[id]}
          </button>
        ))}
      </div>
      {OS_TABS.map((id) => (
        <div key={id} role="tabpanel" id={`os-panel-${id}`} aria-labelledby={`os-${id}`} hidden={tab !== id} tabIndex={0} className="mt-5 outline-none focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary">
          {panels[id]}
        </div>
      ))}
    </div>
  );
}
