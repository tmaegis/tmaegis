"use client";

import type { ReactNode } from "react";
import { Download, PackageX } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { CodeBlock } from "@/components/code-block";
import { CopyButton } from "@/components/copy-button";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PlatformIcon } from "@/components/platform-icon";
import { isExperimental, type Arch, type OS, type ReleaseAsset } from "@/lib/release-assets";
import { archLabel, formatSize, OS_ICON, OS_NAME } from "./format";

/**
 * One downloadable file: what it is, how big, its SHA-256 with a copy button, an optional copy-ready
 * install command for exactly this file, and a button that goes to GitHub.
 */
export function AssetCard({
  asset,
  os,
  arch,
  checksumsUrl,
  primary = false,
  title,
  command,
  commandLang,
  headingLevel = 3,
}: {
  asset: ReleaseAsset;
  os: OS;
  arch: Arch;
  checksumsUrl: string | null;
  primary?: boolean;
  /** Defaults to the operating system name. */
  title?: string;
  command?: string;
  commandLang?: string;
  headingLevel?: 3 | 4;
}) {
  const t = useTranslations("downloads");
  const fmt = useFormatter();
  const label = archLabel(os, arch, t);
  const heading = title ?? OS_NAME[os];
  const experimental = isExperimental(asset.os, asset.arch);
  return (
    <article className="glass group flex min-w-0 flex-col gap-3 p-4" aria-label={`${heading} ${label}`}>
      <div className="flex flex-wrap items-center gap-2">
        <PlatformIcon id={arch === "s390x" ? "s390x" : arch === "riscv64" ? "riscv" : OS_ICON[os]} size={32} tile active={primary} />
        {headingLevel === 4 ? <h4 className="text-base font-semibold">{heading}</h4> : <h3 className="text-base font-semibold">{heading}</h3>}
        <Badge variant={primary ? "accent" : "default"}>{label}</Badge>
        <Badge variant="default" className="font-mono">{asset.format}</Badge>
        {experimental && <Badge variant="medium">{t("experimental")}</Badge>}
        {asset.size !== null && <span className="ms-auto text-sm tabular-nums text-muted-foreground">{formatSize(asset.size, (n, o) => fmt.number(n, o))}</span>}
      </div>
      <p className="break-all font-mono text-xs text-muted-foreground" dir="ltr">{asset.name}</p>
      <div className="min-w-0 rounded-lg border border-border bg-muted/50 p-2.5">
        <div className="flex items-center justify-between gap-2">
          <span className="label-caps">SHA-256</span>
          {asset.sha256 && <CopyButton text={asset.sha256} label={t("sha.copy")} compact tone="themed" />}
        </div>
        {asset.sha256 ? (
          <code className="mt-1 block break-all font-mono text-[0.72rem] leading-snug" dir="ltr">{asset.sha256}</code>
        ) : (
          <p className="mt-1 text-xs text-muted-foreground">
            {t("sha.missing")}{" "}
            {checksumsUrl && <a href={checksumsUrl} className="link" rel="noopener noreferrer">checksums.txt</a>}
          </p>
        )}
      </div>
      {command && (
        <div className="min-w-0">
          <p className="label-caps mb-1.5">{t("os.commandLabel")}</p>
          <CodeBlock code={command} lang={commandLang} />
        </div>
      )}
      <Button asChild size={primary ? "lg" : "default"} className="mt-auto w-full">
        <a href={asset.url} rel="noopener noreferrer" aria-label={t("downloadFile", { name: asset.name })}>
          <Download className="h-4 w-4" aria-hidden />
          {t("download")}
        </a>
      </Button>
    </article>
  );
}

/** The "this file is not part of the release" state: no link and no command for a file that does not exist. */
export function MissingCard({ title, detail, children, icon }: { title: string; detail: string; children?: ReactNode; icon?: ReactNode }) {
  return (
    <div className="glass flex min-w-0 flex-col gap-2 border-dashed p-4 text-sm text-muted-foreground" role="note">
      <p className="flex items-center gap-2 font-medium text-foreground">
        {icon ?? <PackageX className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />}
        {title}
      </p>
      <p>{detail}</p>
      {children}
    </div>
  );
}
