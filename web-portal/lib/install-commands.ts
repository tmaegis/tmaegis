/**
 * Copy-ready commands for every install option on /downloads. Pure string builders (unit-tested in
 * lib/install-commands.test.ts) so the server page and the client components show the same text.
 * They take the real file name of the release asset; the page only calls them for files the release lists.
 */
import { goName, type Arch, type LinuxPackageFormat, type OS } from "./release-assets";

/** Unpack an archive and run a read-only scan, then preview the fixes without changing anything. */
export function archiveCommand(os: OS, file: string): string {
  // AIX and Solaris tar cannot gunzip by itself, and sudo is often not installed: unpack with gzip, run as root.
  if (os === "aix" || os === "solaris") return `mkdir tmaegis && cd tmaegis\ngzip -dc ../${file} | tar -xf -\n# as root:\n./tmaegis scan --auto-detect\n./tmaegis apply --auto-detect --dry-run`;
  if (os === "freebsd") return `mkdir tmaegis && tar -xzf ${file} -C tmaegis && cd tmaegis\n# as root (su, doas or sudo):\n./tmaegis scan --auto-detect\n./tmaegis apply --auto-detect --dry-run`;
  if (os === "windows") return `Expand-Archive .\\${file} -DestinationPath tmaegis\ncd tmaegis\n.\\tmaegis.exe scan --auto-detect\n.\\tmaegis.exe apply --auto-detect --dry-run`;
  if (os === "macos") return `mkdir tmaegis && tar -xzf ${file} -C tmaegis && cd tmaegis\nxattr -d com.apple.quarantine ./tmaegis\n./tmaegis scan --auto-detect\nsudo ./tmaegis apply --auto-detect --dry-run`;
  return `mkdir tmaegis && tar -xzf ${file} -C tmaegis && cd tmaegis\nsudo ./tmaegis scan --auto-detect\nsudo ./tmaegis apply --auto-detect --dry-run`;
}

/** Package managers that install a local .rpm file. */
export type RpmManager = "dnf" | "zypper";

/** Install a downloaded .deb or .rpm with the distribution's package manager, then scan. */
export function packageCommand(format: LinuxPackageFormat, file: string, rpm?: RpmManager): string {
  if (format === "deb") return `sudo apt install ./${file}\nsudo tmaegis scan --auto-detect`;
  if (rpm === "dnf") return `sudo dnf install ./${file}\nsudo tmaegis scan --auto-detect`;
  if (rpm === "zypper") return `sudo zypper install ./${file}\nsudo tmaegis scan --auto-detect`;
  return `# Fedora, RHEL, Rocky Linux, AlmaLinux\nsudo dnf install ./${file}\n# openSUSE, SUSE Linux Enterprise\nsudo zypper install ./${file}\n\nsudo tmaegis scan --auto-detect`;
}

/** Uninstall the package again (the package name is "tmaegis"). */
export function removeCommand(format: LinuxPackageFormat): string {
  return format === "deb" ? "sudo apt remove tmaegis" : "sudo dnf remove tmaegis\n# or: sudo zypper remove tmaegis";
}

/** Pull the image and show the built-in help. */
export function dockerPullCommand(ref: string): string {
  return `docker pull ${ref}\ndocker run --rm ${ref} --help`;
}

/** Scan an exported network-device configuration from the current folder, mounted read-only. */
export function dockerDeviceCommand(ref: string, target = "cisco-ios", file = "running-config.txt"): string {
  return `docker run --rm -v "$PWD:/data:ro" ${ref} \\\n  scan --target ${target} --device-config /data/${file}`;
}

/** The same scan in a CI job, with a SARIF report on standard output. */
export function dockerCiCommand(ref: string, target = "juniper-junos", file = "configs/router.txt"): string {
  return `docker run --rm -v "$PWD:/data:ro" ${ref} \\\n  scan --target ${target} --device-config /data/${file} --format sarif > tmaegis.sarif`;
}

/** Build the image yourself when the registry has none for this release. */
export const DOCKER_BUILD = "git clone https://github.com/tmaegis/tmaegis.git\ncd tmaegis\ndocker build -t tmaegis .\ndocker run --rm tmaegis --help";

/** Cross-compile one build with Go (no C toolchain needed), for a system whose archive is not in the release. */
export function crossBuildCommand(os: OS, arch: Arch): string {
  const [goos, goarch] = goName(os, arch).split("_");
  return `git clone https://github.com/tmaegis/tmaegis.git\ncd tmaegis\nCGO_ENABLED=0 GOOS=${goos} GOARCH=${goarch} go build -trimpath -o tmaegis .\n# copy tmaegis and the rules folder to the ${goos}/${goarch} system`;
}

export const SOURCE_BUILD = "git clone https://github.com/tmaegis/tmaegis.git\ncd tmaegis\nmake build\n./bin/tmaegis scan --auto-detect";

/** The sample configuration path as it appears inside every archive and in the repository. */
export function samplePath(target: string, kind: "compliant" | "noncompliant" = "noncompliant"): string {
  return `samples/network/${target}/${target}-${kind}.txt`;
}

/** Scan the bundled non-compliant sample of a network target, from inside the extracted archive folder. */
export function sampleCommand(target: string, os: "unix" | "windows" = "unix"): string {
  const bin = os === "windows" ? ".\\tmaegis.exe" : "./tmaegis";
  const path = os === "windows" ? samplePath(target).replaceAll("/", "\\") : samplePath(target);
  return `${bin} scan --target ${target} --device-config ${path}`;
}
