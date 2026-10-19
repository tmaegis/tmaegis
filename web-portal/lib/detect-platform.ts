import { archiveName, type Arch, type OS } from "./release-assets";

/** What the browser told us. `archSure` is false when the architecture is a best guess. */
export interface Detected {
  os: OS | null;
  arch: Arch | null;
  archSure: boolean;
  /** Phones and tablets cannot run TMAegis, so no download is recommended. */
  mobile: boolean;
}

/** The subset of `navigator.userAgentData.getHighEntropyValues()` that we read. */
export interface ClientHints {
  platform?: string;
  architecture?: string;
  bitness?: string;
  mobile?: boolean;
}

function osFromHint(platform: string | undefined): OS | null {
  const p = (platform ?? "").toLowerCase();
  if (p.startsWith("win")) return "windows";
  if (p === "macos" || p === "mac os x" || p.startsWith("mac")) return "macos";
  if (p === "linux" || p === "chrome os" || p === "chromeos" || p === "ubuntu") return "linux";
  return null;
}

/** Maps Client Hints architecture + bitness to a build we publish. 32-bit values map to the 64-bit build. */
export function archFromHint(architecture: string | undefined, bitness: string | undefined): Arch | null {
  const a = (architecture ?? "").toLowerCase();
  if (a.startsWith("arm")) return bitness === "32" ? null : "arm64"; // 32-bit ARM has no build
  // Only 64-bit builds are published, so a 32-bit browser on 64-bit Windows still gets the x64 build.
  if (a.startsWith("x86")) return "x64";
  return null;
}

/** Best effort from `navigator.userAgent`. macOS never reveals Apple silicon here, so it is a guess. */
export function detectFromUserAgent(ua: string): Detected {
  const s = ua || "";
  if (/android|iphone|ipad|ipod/i.test(s)) return { os: null, arch: null, archSure: false, mobile: true };
  let os: OS | null = null;
  if (/windows/i.test(s)) os = "windows";
  else if (/mac os x|macintosh/i.test(s)) os = "macos";
  else if (/linux|x11|cros/i.test(s)) os = "linux";

  let arch: Arch | null = null;
  if (/aarch64|arm64|armv8/i.test(s)) arch = "arm64";
  else if (/x86_64|x86-64|x64|amd64|wow64|win64/i.test(s)) arch = "x64";
  if (arch) return { os, arch, archSure: true, mobile: false };
  // Safari and Firefox on every Mac say "Intel Mac OS X". Most Macs in use today are Apple silicon.
  if (os === "macos") return { os, arch: "arm64", archSure: false, mobile: false };
  if (os) return { os, arch: "x64", archSure: false, mobile: false };
  return { os: null, arch: null, archSure: false, mobile: false };
}

/** Client Hints win over the user-agent string; whatever they leave out is filled in from the user agent. */
export function detectPlatform(ua: string, hints?: ClientHints | null): Detected {
  const fromUa = detectFromUserAgent(ua);
  if (!hints) return fromUa;
  if (hints.mobile) return { os: null, arch: null, archSure: false, mobile: true };
  const os = osFromHint(hints.platform) ?? fromUa.os;
  const hintArch = archFromHint(hints.architecture, hints.bitness);
  if (hintArch) return { os, arch: hintArch, archSure: true, mobile: fromUa.mobile && !os };
  if (os && os === fromUa.os) return fromUa;
  return os ? { os, arch: os === "macos" ? "arm64" : "x64", archSure: false, mobile: false } : fromUa;
}

/** Reads the browser. Never throws; resolves to the user-agent result when Client Hints are missing. */
export async function detectInBrowser(nav: Navigator = navigator): Promise<Detected> {
  const ua = nav.userAgent ?? "";
  // iPadOS Safari presents itself as a Mac but has a touch screen.
  const ipad = /macintosh/i.test(ua) && (nav.maxTouchPoints ?? 0) > 1;
  if (ipad) return { os: null, arch: null, archSure: false, mobile: true };
  const uad = (nav as Navigator & { userAgentData?: { mobile?: boolean; platform?: string; getHighEntropyValues?: (keys: string[]) => Promise<ClientHints> } }).userAgentData;
  if (!uad?.getHighEntropyValues) return detectFromUserAgent(ua);
  try {
    const high = await uad.getHighEntropyValues(["architecture", "bitness", "platform"]);
    return detectPlatform(ua, { platform: high.platform || uad.platform, architecture: high.architecture, bitness: high.bitness, mobile: uad.mobile });
  } catch {
    return detectFromUserAgent(ua);
  }
}

/** Linux distribution families that map to a package format. */
export type LinuxFamily = "debian" | "fedora" | "suse";

/**
 * Some Linux browsers name the distribution in the user agent (Firefox on Ubuntu says "X11; Ubuntu; Linux x86_64",
 * Fedora's Firefox says "Fedora"). Most do not, and then this returns null and the page recommends the archive.
 */
export function linuxFamily(ua: string): LinuxFamily | null {
  const s = ua || "";
  if (!/linux|x11/i.test(s) || /android|cros/i.test(s)) return null;
  if (/ubuntu|debian|linux mint|\bmint\b|pop!_os|elementary|kali|raspbian|zorin/i.test(s)) return "debian";
  if (/opensuse|\bsuse\b|\bsles\b/i.test(s)) return "suse";
  if (/fedora|red hat|redhat|\brhel\b|centos|rocky|almalinux/i.test(s)) return "fedora";
  return null;
}

/** The command that unpacks and starts a scan, for the file the visitor is about to download. */
export function runSnippet(os: OS, fileName: string): string {
  if (os === "windows") return `Expand-Archive .\\${fileName} -DestinationPath tmaegis; cd tmaegis; .\\tmaegis.exe scan --auto-detect`;
  const sudo = os === "linux" ? "sudo " : "";
  return `mkdir tmaegis && tar -xzf ${fileName} -C tmaegis && cd tmaegis && ${sudo}./tmaegis scan --auto-detect`;
}

/** File name to show when the release has no matching asset (used for commands only, never as a link). */
export function expectedFileName(version: string, os: OS, arch: Arch): string {
  return archiveName(version, os, arch);
}
