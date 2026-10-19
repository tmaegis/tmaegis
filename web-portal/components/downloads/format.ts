import type { Arch, OS } from "@/lib/release-assets";

export const OS_NAME: Record<OS, string> = { windows: "Windows", macos: "macOS", linux: "Linux", freebsd: "FreeBSD", aix: "IBM AIX", solaris: "Oracle Solaris" };

/** Icon ids (lib/platform-icons.ts) for each system. */
export const OS_ICON: Record<OS, string> = { windows: "windows", macos: "macos", linux: "linux", freebsd: "freebsd", aix: "aix", solaris: "solaris" };

type T = (key: string) => string;

/** "Apple silicon" / "Intel" on macOS, "x64" / "ARM64" elsewhere. Product names stay as they are. */
export function archLabel(os: OS, arch: Arch, t: T): string {
  if (os === "macos") return arch === "arm64" ? t("arch.macosArm64") : t("arch.macosX64");
  if (arch === "ppc64" || arch === "s390x" || arch === "riscv64") return t(`arch.${arch}`);
  return arch === "arm64" ? t("arch.arm64") : t("arch.x64");
}

/** Binary units (what file managers show), formatted with the visitor's number format. */
export function formatSize(bytes: number, format: (n: number, o: { maximumFractionDigits: number }) => string): string {
  if (bytes >= 1048576) return `${format(bytes / 1048576, { maximumFractionDigits: 1 })} MiB`;
  return `${format(Math.max(1, bytes / 1024), { maximumFractionDigits: 0 })} KiB`;
}
