/**
 * Pure helpers that turn GitHub Release assets into a typed download model. No I/O here, so the
 * parsing rules are unit-tested (lib/release-assets.test.ts) and shared by the server and client.
 *
 * GoReleaser (.goreleaser.yaml) names the archives `tmaegis_<version>_<os>_<arch>.<zip|tar.gz>`, builds the
 * Linux packages `tmaegis_<version>_linux_<arch>.<deb|rpm>` with nfpm (RPMs may say x86_64/aarch64) and
 * writes `checksums.txt` (SHA-256). Installers, per-file checksums, signatures and SBOMs are recognised as
 * well so the page keeps working if the release grows. Nothing here assumes a file exists: the page shows
 * "not in this release" for every file a release does not list.
 */

export type OS = "windows" | "macos" | "linux" | "freebsd" | "aix" | "solaris";
export type Arch = "x64" | "arm64" | "x86" | "arm" | "ppc64" | "s390x" | "riscv64";
export type AssetKind = "archive" | "installer" | "package" | "checksums" | "checksum" | "signature" | "sbom" | "other";
export type Component = "tmaegis" | "tmaegis-server";

export const OS_ORDER: readonly OS[] = ["windows", "macos", "linux", "freebsd", "aix", "solaris"];
/** The three systems with stable builds (the "For you" picker and the main tabs). */
export const STABLE_OS: readonly OS[] = ["windows", "macos", "linux"];
export const ARCH_ORDER: readonly Arch[] = ["x64", "arm64", "x86", "arm", "ppc64", "s390x", "riscv64"];
/** The builds GoReleaser publishes for every release. */
export const SUPPORTED: readonly { os: OS; arch: Arch }[] = [
  { os: "windows", arch: "x64" },
  { os: "windows", arch: "arm64" },
  { os: "macos", arch: "arm64" },
  { os: "macos", arch: "x64" },
  { os: "linux", arch: "x64" },
  { os: "linux", arch: "arm64" },
];

/**
 * Experimental builds (the `tmaegis-extra` build in .goreleaser.yaml): compile-checked only, never run on real
 * hardware. The page labels them "Experimental" and offers the cross-compile command when one is missing.
 */
export const EXPERIMENTAL: readonly { os: OS; arch: Arch }[] = [
  { os: "freebsd", arch: "x64" },
  { os: "freebsd", arch: "arm64" },
  { os: "aix", arch: "ppc64" },
  { os: "solaris", arch: "x64" },
  { os: "linux", arch: "s390x" },
  { os: "linux", arch: "riscv64" },
];

export function isExperimental(os: OS | null, arch: Arch | null): boolean {
  return !!os && !!arch && EXPERIMENTAL.some((e) => e.os === os && e.arch === arch);
}

export interface ReleaseAsset {
  name: string;
  url: string;
  /** Bytes, or null when unknown (bundled fallback before the release was built). */
  size: number | null;
  kind: AssetKind;
  /** File format: zip, tar.gz, msi, deb, rpm, pkg, txt, sha256, sig, sbom ... */
  format: string;
  os: OS | null;
  arch: Arch | null;
  component: Component;
  version: string | null;
  /** Lower-case hex SHA-256, or null when the release has no checksum for this file. */
  sha256: string | null;
}

export interface ReleaseSummary {
  tag: string;
  name: string;
  publishedAt: string | null;
  url: string;
  prerelease: boolean;
}

export interface Release extends ReleaseSummary {
  /** Version without the leading "v". */
  version: string;
  /** Release notes (GitHub markdown), possibly empty. */
  body: string;
  assets: ReleaseAsset[];
}

export interface RawAsset {
  name: string;
  url: string;
  size?: number | null;
  sha256?: string | null;
}

/** GitHub's REST representation, reduced to the fields we read. */
export interface GitHubReleaseJson {
  tag_name?: string;
  name?: string | null;
  body?: string | null;
  published_at?: string | null;
  html_url?: string;
  draft?: boolean;
  prerelease?: boolean;
  assets?: { name?: string; size?: number; browser_download_url?: string }[];
}

const FORMATS: [RegExp, string, AssetKind][] = [
  [/(?:^|[._-])(?:checksums?|sha256sums)(?:\.txt)?$/i, "txt", "checksums"],
  [/\.sha256(?:sum)?$/i, "sha256", "checksum"],
  [/\.(?:sig|asc|pem|cert|bundle)$/i, "sig", "signature"],
  [/(?:\.sbom|\.spdx|\.cdx|\.cyclonedx)(?:\.json|\.xml)?$/i, "sbom", "sbom"],
  [/\.tar\.gz$|\.tgz$/i, "tar.gz", "archive"],
  [/\.tar\.xz$/i, "tar.xz", "archive"],
  [/\.zip$/i, "zip", "archive"],
  [/\.msi$/i, "msi", "installer"],
  [/\.exe$/i, "exe", "installer"],
  [/\.pkg$/i, "pkg", "installer"],
  [/\.dmg$/i, "dmg", "installer"],
  [/\.deb$/i, "deb", "package"],
  [/\.rpm$/i, "rpm", "package"],
  [/\.apk$/i, "apk", "package"],
];

/** Parses an asset file name. Unknown names come back as kind "other" with null os/arch. */
export function parseAssetName(name: string): Omit<ReleaseAsset, "url" | "size" | "sha256"> {
  let format = name.includes(".") ? name.slice(name.lastIndexOf(".") + 1).toLowerCase() : "";
  let kind: AssetKind = "other";
  for (const [re, f, k] of FORMATS) {
    if (re.test(name)) {
      format = f;
      kind = k;
      break;
    }
  }
  // Signatures and per-file checksums describe the file they belong to, so parse that name.
  const subject = kind === "signature" || kind === "checksum" ? name.replace(/\.[^.]+$/, "") : name;
  const tokens = subject.toLowerCase().split(/[_.\-\s]+/);
  const has = (...xs: string[]) => xs.some((x) => tokens.includes(x));

  let os: OS | null = has("windows", "win", "win64", "win32")
    ? "windows"
    : has("darwin", "macos", "osx", "mac")
      ? "macos"
      : has("linux")
        ? "linux"
        : has("freebsd")
          ? "freebsd"
          : has("aix")
            ? "aix"
            : has("solaris", "sunos")
              ? "solaris"
              : null;
  if (!os && kind !== "checksums") {
    if (/\.(?:deb|rpm|apk)$/i.test(subject)) os = "linux";
    else if (/\.(?:msi|exe)$/i.test(subject)) os = "windows";
    else if (/\.(?:pkg|dmg)$/i.test(subject)) os = "macos";
  }
  const arch: Arch | null = has("arm64", "aarch64")
    ? "arm64"
    : // the token split turns "x86_64" into "x86" + "64", so test the raw name for it
      has("amd64", "x64") || /x86[_-]64/i.test(subject)
      ? "x64"
      : has("ppc64")
        ? "ppc64"
        : has("s390x")
          ? "s390x"
          : has("riscv64")
            ? "riscv64"
            : has("386", "i386", "i686", "x86")
              ? "x86"
              : has("armv7", "armv6", "armhf", "arm")
                ? "arm"
                : null;
  // Architecture words are not part of a pre-release suffix (`tmaegis-0.1.0-1.x86_64.rpm`), and the RPM
  // release number after the version (`-1`) is not part of the version either.
  const bare = subject.replace(/[._-](?:x86_64|aarch64|amd64|arm64|noarch|i[36]86|armv7hl|armhf|ppc64|s390x|riscv64)(?=[._-]|$)/gi, "_");
  let version = bare.match(/[_-]v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)(?=[_.-]|$)/)?.[1]?.replace(/\.$/, "") ?? null;
  if (version && format === "rpm") version = version.replace(/-\d+$/, "");
  const component: Component = /^tmaegis[-_]server/i.test(subject) ? "tmaegis-server" : "tmaegis";
  return { name, kind, format, os: kind === "checksums" ? null : os, arch: kind === "checksums" ? null : arch, component, version };
}

/** Parses `sha256sum` output (`<hex>  <name>` or `<hex> *<name>`) into name -> lower-case hex. */
export function parseChecksums(text: string): Map<string, string> {
  const out = new Map<string, string>();
  for (const line of text.split(/\r?\n/)) {
    const m = line.trim().match(/^([a-fA-F0-9]{64})\s+\*?(.+)$/);
    if (m) out.set(m[2].trim().replace(/^\.\//, ""), m[1].toLowerCase());
  }
  return out;
}

/** A per-file `<file>.sha256` contains just the hash, or `sha256sum` output for that file. */
export function parseSingleChecksum(text: string): string | null {
  const m = text.trim().match(/^([a-fA-F0-9]{64})\b/);
  return m ? m[1].toLowerCase() : null;
}

/** Only files served by GitHub Releases are ever linked (never this site, never a third party). */
export function isTrustedDownloadUrl(url: string): boolean {
  try {
    const u = new URL(url);
    return u.protocol === "https:" && u.hostname === "github.com" && /^\/[^/]+\/[^/]+\/releases\/download\//.test(u.pathname);
  } catch {
    return false;
  }
}

/** Builds the typed asset list; hashes come from the asset itself or from checksums.txt. */
export function buildAssets(raw: RawAsset[], checksums?: Map<string, string>): ReleaseAsset[] {
  return raw
    .filter((a) => a.name && isTrustedDownloadUrl(a.url))
    .map((a) => {
      const p = parseAssetName(a.name);
      const hash = (a.sha256 && /^[a-f0-9]{64}$/i.test(a.sha256) ? a.sha256.toLowerCase() : null) ?? checksums?.get(a.name) ?? null;
      return { ...p, url: a.url, size: typeof a.size === "number" && a.size > 0 ? a.size : null, sha256: hash };
    })
    .sort(compareAssets);
}

const KIND_ORDER: AssetKind[] = ["archive", "installer", "package", "checksums", "checksum", "signature", "sbom", "other"];
const rank = <T>(list: readonly T[], v: T | null) => (v === null ? list.length : list.indexOf(v) === -1 ? list.length : list.indexOf(v));

export function compareAssets(a: ReleaseAsset, b: ReleaseAsset): number {
  return (
    rank(KIND_ORDER, a.kind) - rank(KIND_ORDER, b.kind) ||
    rank(OS_ORDER, a.os) - rank(OS_ORDER, b.os) ||
    rank(ARCH_ORDER, a.arch) - rank(ARCH_ORDER, b.arch) ||
    a.name.localeCompare(b.name, "en")
  );
}

/** The main download for a platform: the archive, else an installer, else a package. */
export function findPackage(assets: ReleaseAsset[], os: OS, arch: Arch): ReleaseAsset | null {
  const mine = assets.filter((a) => a.os === os && a.arch === arch && a.component === "tmaegis");
  for (const k of ["archive", "installer", "package"] as const) {
    const hit = mine.find((a) => a.kind === k);
    if (hit) return hit;
  }
  return null;
}

/** Installers and packages for a platform besides the main archive (msi, pkg, deb, rpm ...). */
export function extraPackages(assets: ReleaseAsset[], os: OS): ReleaseAsset[] {
  return assets.filter((a) => a.os === os && (a.kind === "installer" || a.kind === "package"));
}

export function checksumsAsset(assets: ReleaseAsset[]): ReleaseAsset | null {
  return assets.find((a) => a.kind === "checksums") ?? null;
}

/** Normalises a GitHub REST release; null for drafts or malformed data. */
export function fromGitHub(j: GitHubReleaseJson | null | undefined, repo: string): { release: Omit<Release, "assets">; raw: RawAsset[] } | null {
  if (!j || typeof j !== "object" || !j.tag_name || j.draft) return null;
  const tag = j.tag_name;
  return {
    release: {
      tag,
      version: tag.replace(/^v/, ""),
      name: j.name || tag,
      publishedAt: j.published_at ?? null,
      url: j.html_url && j.html_url.startsWith("https://github.com/") ? j.html_url : `https://github.com/${repo}/releases/tag/${tag}`,
      prerelease: !!j.prerelease,
      body: typeof j.body === "string" ? j.body : "",
    },
    raw: (j.assets ?? []).flatMap((a) => (a.name && a.browser_download_url ? [{ name: a.name, url: a.browser_download_url, size: a.size ?? null }] : [])),
  };
}

export function summaryOf(r: ReleaseSummary): ReleaseSummary {
  return { tag: r.tag, name: r.name, publishedAt: r.publishedAt, url: r.url, prerelease: r.prerelease };
}

/** `tmaegis_0.1.0_windows_amd64.zip` -> the GOOS/GOARCH spelling used in file names. */
export function goName(os: OS, arch: Arch): string {
  const goos = os === "macos" ? "darwin" : os;
  const goarch = arch === "x64" ? "amd64" : arch === "x86" ? "386" : arch;
  return `${goos}_${goarch}`;
}

export function archiveName(version: string, os: OS, arch: Arch): string {
  return `tmaegis_${version.replace(/^v/, "")}_${goName(os, arch)}.${os === "windows" ? "zip" : "tar.gz"}`;
}

/** Linux package formats GoReleaser builds with nfpm. */
export type LinuxPackageFormat = "deb" | "rpm";
export const LINUX_PACKAGE_FORMATS: readonly LinuxPackageFormat[] = ["deb", "rpm"];

/** The .deb or .rpm for one architecture, whatever spelling of the architecture the file name uses. */
export function findLinuxPackage(assets: ReleaseAsset[], format: LinuxPackageFormat, arch: Arch): ReleaseAsset | null {
  return assets.find((a) => a.kind === "package" && a.format === format && a.os === "linux" && a.arch === arch && a.component === "tmaegis") ?? null;
}

/** `tmaegis_0.1.0_linux_amd64.deb`: the planned name, for the bundled fallback and for commands (never a link by itself). */
export function linuxPackageName(version: string, format: LinuxPackageFormat, arch: Arch): string {
  return `tmaegis_${version.replace(/^v/, "")}_${goName("linux", arch)}.${format}`;
}

/** The multi-arch container image GoReleaser pushes to GitHub Container Registry for every release. */
export function containerImage(repo: string): string {
  return `ghcr.io/${repo.toLowerCase()}`;
}

/** Image tags are the version without the leading "v" (`ghcr.io/tmaegis/tmaegis:0.1.0`). */
export function imageTag(version: string): string {
  return version.replace(/^v/, "");
}
