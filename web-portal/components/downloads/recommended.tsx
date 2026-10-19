"use client";

import { useEffect, useState } from "react";
import { ArrowDown, Info } from "lucide-react";
import { useTranslations } from "next-intl";
import { CodeBlock } from "@/components/code-block";
import { Button } from "@/components/ui/button";
import { detectInBrowser, linuxFamily, runSnippet, type Detected, type LinuxFamily } from "@/lib/detect-platform";
import { packageCommand } from "@/lib/install-commands";
import { findLinuxPackage, findPackage, STABLE_OS, SUPPORTED, type Arch, type OS, type ReleaseAsset } from "@/lib/release-assets";
import { AssetCard } from "./asset-card";
import { PlatformIcon } from "@/components/platform-icon";
import { archLabel, OS_ICON, OS_NAME } from "./format";
import { Segment } from "./segment";

const ARCHES: Arch[] = ["x64", "arm64"];

/**
 * Detects the visitor's OS and CPU in the browser and puts the matching download first. On Linux, when the
 * browser names the distribution (rare), the .deb or .rpm comes first; otherwise the archive, which works everywhere.
 */
export function Recommended({ assets, checksumsUrl }: { assets: ReleaseAsset[]; checksumsUrl: string | null }) {
  const t = useTranslations("downloads");
  const [detected, setDetected] = useState<Detected | null>(null);
  const [os, setOs] = useState<OS | null>(null);
  const [arch, setArch] = useState<Arch | null>(null);
  const [touched, setTouched] = useState(false);
  const [family, setFamily] = useState<LinuxFamily | null>(null);

  useEffect(() => {
    let alive = true;
    detectInBrowser().then((d) => {
      if (!alive) return;
      setDetected(d);
      setOs(d.os);
      setArch(d.arch);
      setFamily(linuxFamily(navigator.userAgent ?? ""));
    });
    return () => {
      alive = false;
    };
  }, []);

  const checksums = checksumsUrl;
  const archive = os && arch ? findPackage(assets, os, arch) : null;
  // Only a package that this release really lists is ever recommended.
  const pkgFormat = os === "linux" && family ? (family === "debian" ? "deb" : "rpm") : null;
  const pkg = pkgFormat && arch ? findLinuxPackage(assets, pkgFormat, arch) : null;
  const pick = pkg ?? archive;
  // With a package recommended, the archive is the one alternative worth showing; otherwise list installers and packages.
  const extras = !os || !arch || !pick ? [] : pkg ? (archive ? [archive] : []) : assets.filter((a) => a.os === os && a.arch === arch && a !== pick && (a.kind === "installer" || a.kind === "package"));
  const pickCommand = pick && os ? (pick.kind === "package" && (pick.format === "deb" || pick.format === "rpm") ? packageCommand(pick.format, pick.name, family === "suse" ? "zypper" : family === "fedora" ? "dnf" : undefined) : runSnippet(os, pick.name)) : "";
  const archOptions = ARCHES.map((a) => ({ id: a, text: os ? archLabel(os, a, t) : a }));
  const osOptions = STABLE_OS.map((o) => ({ id: o, text: OS_NAME[o], icon: <PlatformIcon id={OS_ICON[o]} size={16} /> }));
  const supportedHere = os && arch ? SUPPORTED.some((s) => s.os === os && s.arch === arch) : false;

  let status: string;
  if (!detected) status = t("recommended.detecting");
  else if (detected.mobile && !touched) status = t("recommended.mobile");
  else if (!os || !arch) status = t("recommended.unknown");
  else if (touched) status = t("recommended.chosen", { os: OS_NAME[os], arch: archLabel(os, arch, t) });
  else status = t("recommended.detected", { os: OS_NAME[os], arch: archLabel(os, arch, t) });

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
      <div className="min-w-0 space-y-4">
        <p role="status" aria-live="polite" className="text-muted-foreground" data-testid="detect-status">{status}</p>
        {detected && !touched && !detected.archSure && os && arch && <p className="flex items-start gap-2 text-sm text-muted-foreground"><Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden />{t("recommended.guessed")}</p>}
        {pkg && <p className="flex items-start gap-2 text-sm text-muted-foreground"><Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden />{pkg.format === "deb" ? t("recommended.linuxDeb") : t("recommended.linuxRpm")}</p>}

        <div className="space-y-3">
          <p className="text-sm font-medium">{t("recommended.override")}</p>
          <div className="flex flex-wrap items-center gap-3">
            <Segment label={t("recommended.osLabel")} value={os} options={osOptions} onChange={(v) => { setOs(v); setArch((a) => a ?? (v === "macos" ? "arm64" : "x64")); setTouched(true); }} />
            <Segment label={t("recommended.archLabel")} value={arch} options={archOptions} onChange={(v) => { setArch(v); setOs((o) => o ?? "windows"); setTouched(true); }} />
          </div>
        </div>

        {pick && os && arch && (
          <div className="space-y-3">
            <p className="text-sm font-medium">{pkg ? t("recommended.runPackage") : t("recommended.runIt")}</p>
            <CodeBlock code={pickCommand} lang={os === "windows" ? "powershell" : "shell"} />
            <p className="text-xs text-muted-foreground">{pkg ? t("recommended.runPackageNote") : os === "windows" ? t("recommended.runWindows") : os === "macos" ? t("recommended.runMac") : t("recommended.runLinux")}</p>
          </div>
        )}
      </div>

      <div className="min-w-0 space-y-3" data-testid="recommended-card">
        {pick && os && arch ? (
          <>
            <AssetCard asset={pick} os={os} arch={arch} checksumsUrl={checksums} primary />
            {extras.length > 0 && (
              <ul className="space-y-2">
                {extras.map((x) => (
                  <li key={x.name}><AssetCard asset={x} os={os} arch={arch} checksumsUrl={checksums} /></li>
                ))}
              </ul>
            )}
          </>
        ) : (
          <div className="glass space-y-3 p-5">
            <p className="font-medium">{os && arch && !supportedHere ? t("recommended.noBuild", { os: OS_NAME[os], arch: archLabel(os, arch, t) }) : t("recommended.pickBelow")}</p>
            <Button asChild variant="outline"><a href="#os"><ArrowDown className="h-4 w-4" aria-hidden />{t("recommended.allPlatforms")}</a></Button>
          </div>
        )}
      </div>
    </div>
  );
}
