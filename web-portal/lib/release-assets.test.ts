import { describe, expect, it } from "vitest";
import {
  archiveName,
  buildAssets,
  checksumsAsset,
  containerImage,
  extraPackages,
  findLinuxPackage,
  EXPERIMENTAL,
  findPackage,
  fromGitHub,
  goName,
  imageTag,
  isExperimental,
  isTrustedDownloadUrl,
  linuxPackageName,
  SUPPORTED,
  parseAssetName,
  parseChecksums,
  parseSingleChecksum,
} from "./release-assets";

const base = "https://github.com/tmaegis/tmaegis/releases/download/v0.1.0/";
const HASH = "a".repeat(64);

describe("parseAssetName", () => {
  it.each([
    ["tmaegis_0.1.0_windows_amd64.zip", "windows", "x64", "zip"],
    ["tmaegis_0.1.0_windows_arm64.zip", "windows", "arm64", "zip"],
    ["tmaegis_0.1.0_darwin_arm64.tar.gz", "macos", "arm64", "tar.gz"],
    ["tmaegis_0.1.0_darwin_amd64.tar.gz", "macos", "x64", "tar.gz"],
    ["tmaegis_0.1.0_linux_amd64.tar.gz", "linux", "x64", "tar.gz"],
    ["tmaegis_0.1.0_linux_arm64.tar.gz", "linux", "arm64", "tar.gz"],
  ] as const)("%s", (name, os, arch, format) => {
    const a = parseAssetName(name);
    expect(a).toMatchObject({ os, arch, format, kind: "archive", version: "0.1.0", component: "tmaegis" });
  });

  it("recognises checksums.txt without a platform", () => {
    expect(parseAssetName("checksums.txt")).toMatchObject({ kind: "checksums", os: null, arch: null, format: "txt" });
  });

  it("recognises installers, packages, signatures and SBOMs", () => {
    expect(parseAssetName("tmaegis_0.1.0_windows_amd64.msi")).toMatchObject({ kind: "installer", os: "windows", arch: "x64", format: "msi" });
    expect(parseAssetName("tmaegis_0.1.0_linux_amd64.deb")).toMatchObject({ kind: "package", os: "linux", arch: "x64", format: "deb" });
    expect(parseAssetName("tmaegis-0.1.0-1.x86_64.rpm")).toMatchObject({ kind: "package", os: "linux", arch: "x64", format: "rpm" });
    expect(parseAssetName("tmaegis_0.1.0_darwin_arm64.pkg")).toMatchObject({ kind: "installer", os: "macos", arch: "arm64", format: "pkg" });
    expect(parseAssetName("tmaegis_0.1.0_linux_amd64.tar.gz.sig")).toMatchObject({ kind: "signature", os: "linux", arch: "x64" });
    expect(parseAssetName("tmaegis_0.1.0_linux_amd64.tar.gz.sha256")).toMatchObject({ kind: "checksum", os: "linux" });
    expect(parseAssetName("tmaegis_0.1.0_linux_amd64.sbom.json")).toMatchObject({ kind: "sbom", os: "linux" });
  });

  it.each([
    ["tmaegis_0.1.0_linux_amd64.deb", "deb", "x64"],
    ["tmaegis_0.1.0_linux_arm64.deb", "deb", "arm64"],
    ["tmaegis_0.1.0_linux_amd64.rpm", "rpm", "x64"],
    ["tmaegis_0.1.0_linux_arm64.rpm", "rpm", "arm64"],
    ["tmaegis_0.1.0_linux_x86_64.rpm", "rpm", "x64"],
    ["tmaegis_0.1.0_linux_aarch64.rpm", "rpm", "arm64"],
    ["tmaegis-0.1.0-1.x86_64.rpm", "rpm", "x64"],
    ["tmaegis-0.1.0-1.aarch64.rpm", "rpm", "arm64"],
    ["tmaegis_0.1.0_amd64.deb", "deb", "x64"],
  ] as const)("Linux package %s", (name, format, arch) => {
    expect(parseAssetName(name)).toMatchObject({ kind: "package", os: "linux", arch, format, version: "0.1.0", component: "tmaegis" });
  });

  it("keeps pre-release versions and does not mistake architectures for them", () => {
    expect(parseAssetName("tmaegis_0.2.0-rc.1_linux_amd64.tar.gz").version).toBe("0.2.0-rc.1");
    expect(parseAssetName("tmaegis_0.2.0-rc.1_linux_x86_64.rpm").version).toBe("0.2.0-rc.1");
  });

  it("tells the server binary apart", () => {
    expect(parseAssetName("tmaegis-server_0.1.0_linux_amd64.tar.gz").component).toBe("tmaegis-server");
  });

  it("falls back to kind other for unknown files", () => {
    expect(parseAssetName("notes.pdf")).toMatchObject({ kind: "other", os: null, arch: null });
  });

  it("round-trips with archiveName", () => {
    expect(archiveName("v0.1.0", "windows", "x64")).toBe("tmaegis_0.1.0_windows_amd64.zip");
    expect(archiveName("0.1.0", "macos", "arm64")).toBe("tmaegis_0.1.0_darwin_arm64.tar.gz");
    expect(archiveName("0.1.0", "linux", "x64")).toBe("tmaegis_0.1.0_linux_amd64.tar.gz");
  });
});

describe("experimental builds", () => {
  it.each([
    ["tmaegis_0.1.0_freebsd_amd64.tar.gz", "freebsd", "x64"],
    ["tmaegis_0.1.0_freebsd_arm64.tar.gz", "freebsd", "arm64"],
    ["tmaegis_0.1.0_aix_ppc64.tar.gz", "aix", "ppc64"],
    ["tmaegis_0.1.0_solaris_amd64.tar.gz", "solaris", "x64"],
    ["tmaegis_0.1.0_linux_s390x.tar.gz", "linux", "s390x"],
    ["tmaegis_0.1.0_linux_riscv64.tar.gz", "linux", "riscv64"],
  ] as const)("%s", (name, os, arch) => {
    expect(parseAssetName(name)).toMatchObject({ os, arch, kind: "archive", format: "tar.gz", version: "0.1.0", component: "tmaegis" });
    expect(isExperimental(os, arch)).toBe(true);
    expect(archiveName("v0.1.0", os, arch)).toBe(name);
  });

  it("keeps the stable builds and the experimental ones apart", () => {
    for (const s of SUPPORTED) expect(isExperimental(s.os, s.arch), `${s.os}/${s.arch}`).toBe(false);
    expect(EXPERIMENTAL).toHaveLength(6);
    expect(isExperimental(null, "x64")).toBe(false);
    expect(goName("solaris", "x64")).toBe("solaris_amd64");
    expect(goName("linux", "riscv64")).toBe("linux_riscv64");
  });

  it("does not take a ppc64le, sunos or s390x file for something else", () => {
    expect(parseAssetName("tmaegis_0.1.0_sunos_amd64.tar.gz").os).toBe("solaris");
    expect(parseAssetName("tmaegis_0.1.0_linux_ppc64le.tar.gz").arch).toBeNull();
    expect(parseAssetName("tmaegis_0.2.0-rc.1_linux_s390x.tar.gz").version).toBe("0.2.0-rc.1");
  });

  it("finds an experimental archive and sorts it after the stable systems", () => {
    const assets = buildAssets([
      { name: "tmaegis_0.1.0_aix_ppc64.tar.gz", url: base + "aix" },
      { name: "tmaegis_0.1.0_freebsd_amd64.tar.gz", url: base + "fb" },
      { name: "tmaegis_0.1.0_linux_amd64.tar.gz", url: base + "lx" },
    ]);
    expect(assets.map((a) => a.os)).toEqual(["linux", "freebsd", "aix"]);
    expect(findPackage(assets, "aix", "ppc64")?.name).toBe("tmaegis_0.1.0_aix_ppc64.tar.gz");
    expect(findPackage(assets, "solaris", "x64")).toBeNull();
  });
});

describe("checksums", () => {
  const text = `${"1".repeat(64)}  tmaegis_0.1.0_linux_amd64.tar.gz\n${"B".repeat(64)} *tmaegis_0.1.0_windows_amd64.zip\r\n\nnot a line\n${"2".repeat(64)}  ./tmaegis_0.1.0_darwin_arm64.tar.gz\n`;
  it("parses sha256sum output, text and binary markers and CRLF", () => {
    const m = parseChecksums(text);
    expect(m.size).toBe(3);
    expect(m.get("tmaegis_0.1.0_linux_amd64.tar.gz")).toBe("1".repeat(64));
    expect(m.get("tmaegis_0.1.0_windows_amd64.zip")).toBe("b".repeat(64));
    expect(m.get("tmaegis_0.1.0_darwin_arm64.tar.gz")).toBe("2".repeat(64));
  });
  it("ignores malformed hashes", () => {
    expect(parseChecksums("abc  file.zip\n").size).toBe(0);
    expect(parseChecksums("").size).toBe(0);
  });
  it("reads a single-file checksum", () => {
    expect(parseSingleChecksum(`${HASH}  file.zip\n`)).toBe(HASH);
    expect(parseSingleChecksum("nope")).toBeNull();
  });
});

describe("buildAssets", () => {
  const raw = [
    { name: "tmaegis_0.1.0_linux_amd64.tar.gz", url: base + "tmaegis_0.1.0_linux_amd64.tar.gz", size: 123 },
    { name: "tmaegis_0.1.0_windows_amd64.zip", url: base + "tmaegis_0.1.0_windows_amd64.zip", size: 0 },
    { name: "checksums.txt", url: base + "checksums.txt", size: 10 },
    { name: "evil.zip", url: "https://evil.example/evil.zip", size: 1 },
    { name: "tmaegis_0.1.0_linux_arm64.tar.gz", url: "/releases/tmaegis_0.1.0_linux_arm64.tar.gz", size: 1 },
  ];
  const sums = new Map([["tmaegis_0.1.0_linux_amd64.tar.gz", HASH]]);

  it("keeps only GitHub release URLs (never this site or a third party)", () => {
    const names = buildAssets(raw).map((a) => a.name);
    expect(names).not.toContain("evil.zip");
    expect(names).not.toContain("tmaegis_0.1.0_linux_arm64.tar.gz");
    expect(names).toHaveLength(3);
  });
  it("attaches size and sha256, null when unknown", () => {
    const a = buildAssets(raw, sums);
    const linux = a.find((x) => x.os === "linux")!;
    expect(linux).toMatchObject({ size: 123, sha256: HASH, kind: "archive" });
    const win = a.find((x) => x.os === "windows")!;
    expect(win.sha256).toBeNull();
    expect(win.size).toBeNull();
  });
  it("sorts archives before checksums, Windows before macOS before Linux", () => {
    const order = buildAssets(raw, sums).map((a) => a.name);
    expect(order).toEqual(["tmaegis_0.1.0_windows_amd64.zip", "tmaegis_0.1.0_linux_amd64.tar.gz", "checksums.txt"]);
  });
  it("trusts an inline sha256 only when it is a real hash", () => {
    const a = buildAssets([{ name: "tmaegis_0.1.0_linux_amd64.tar.gz", url: base + "x.tar.gz", sha256: "zz" }], sums);
    expect(a[0].sha256).toBe(HASH);
  });
});

describe("lookups", () => {
  const assets = buildAssets(
    [
      { name: "tmaegis_0.1.0_linux_amd64.tar.gz", url: base + "a" },
      { name: "tmaegis_0.1.0_linux_amd64.deb", url: base + "b" },
      { name: "tmaegis_0.1.0_windows_amd64.msi", url: base + "c" },
      { name: "checksums.txt", url: base + "d" },
    ],
  );
  it("prefers the archive, then installer, then package", () => {
    expect(findPackage(assets, "linux", "x64")?.format).toBe("tar.gz");
    expect(findPackage(assets, "windows", "x64")?.format).toBe("msi");
    expect(findPackage(assets, "macos", "arm64")).toBeNull();
  });
  it("lists extra packages and the checksums file", () => {
    expect(extraPackages(assets, "linux").map((a) => a.format)).toEqual(["deb"]);
    expect(checksumsAsset(assets)?.name).toBe("checksums.txt");
  });
});

describe("Linux packages and the container image", () => {
  const assets = buildAssets(
    [
      { name: "tmaegis_0.1.0_linux_amd64.tar.gz", url: base + "a" },
      { name: "tmaegis_0.1.0_linux_amd64.deb", url: base + "b" },
      { name: "tmaegis_0.1.0_linux_x86_64.rpm", url: base + "c" },
      { name: "tmaegis_0.1.0_linux_aarch64.rpm", url: base + "d" },
    ],
  );
  it("finds the .deb and .rpm per architecture whatever the spelling", () => {
    expect(findLinuxPackage(assets, "deb", "x64")?.name).toBe("tmaegis_0.1.0_linux_amd64.deb");
    expect(findLinuxPackage(assets, "deb", "arm64")).toBeNull();
    expect(findLinuxPackage(assets, "rpm", "x64")?.name).toBe("tmaegis_0.1.0_linux_x86_64.rpm");
    expect(findLinuxPackage(assets, "rpm", "arm64")?.name).toBe("tmaegis_0.1.0_linux_aarch64.rpm");
  });
  it("names the planned packages and the image", () => {
    expect(linuxPackageName("v0.1.0", "deb", "x64")).toBe("tmaegis_0.1.0_linux_amd64.deb");
    expect(linuxPackageName("0.1.0", "rpm", "arm64")).toBe("tmaegis_0.1.0_linux_arm64.rpm");
    expect(parseAssetName(linuxPackageName("0.1.0", "rpm", "arm64"))).toMatchObject({ kind: "package", arch: "arm64", format: "rpm" });
    expect(containerImage("TMAegis/tmaegis")).toBe("ghcr.io/tmaegis/tmaegis");
    expect(imageTag("v0.1.0")).toBe("0.1.0");
  });
});

describe("isTrustedDownloadUrl", () => {
  it("accepts only https GitHub release downloads", () => {
    expect(isTrustedDownloadUrl(base + "a.zip")).toBe(true);
    expect(isTrustedDownloadUrl("http://github.com/a/b/releases/download/v1/a.zip")).toBe(false);
    expect(isTrustedDownloadUrl("https://github.com.evil.example/a/b/releases/download/v1/a.zip")).toBe(false);
    expect(isTrustedDownloadUrl("https://github.com/a/b/archive/main.zip")).toBe(false);
    expect(isTrustedDownloadUrl("/releases/a.zip")).toBe(false);
    expect(isTrustedDownloadUrl("not a url")).toBe(false);
  });
});

describe("fromGitHub", () => {
  it("normalises a release and skips drafts and junk", () => {
    const r = fromGitHub({ tag_name: "v0.1.0", name: null, body: "hi", html_url: "https://github.com/tmaegis/tmaegis/releases/tag/v0.1.0", assets: [{ name: "a.zip", size: 5, browser_download_url: base + "a.zip" }, { name: "b" }] }, "tmaegis/tmaegis");
    expect(r?.release).toMatchObject({ tag: "v0.1.0", version: "0.1.0", name: "v0.1.0", body: "hi", prerelease: false });
    expect(r?.raw).toHaveLength(1);
    expect(fromGitHub({ tag_name: "v1", draft: true }, "a/b")).toBeNull();
    expect(fromGitHub(null, "a/b")).toBeNull();
    expect(fromGitHub({ tag_name: "v1", html_url: "https://evil.example/x" }, "a/b")?.release.url).toBe("https://github.com/a/b/releases/tag/v1");
  });
});
