import { describe, expect, it } from "vitest";
import { checkImage, fallbackRelease, loadReleaseData, MAX_PREVIOUS } from "./releases";
import { EXPERIMENTAL, findLinuxPackage, isTrustedDownloadUrl, SUPPORTED, findPackage } from "./release-assets";

const base = "https://github.com/tmaegis/tmaegis/releases/download";
const H = (c: string) => c.repeat(64);

function release(tag: string, extra: object = {}) {
  const v = tag.replace(/^v/, "");
  const names = ["windows_amd64.zip", "windows_arm64.zip", "darwin_arm64.tar.gz", "darwin_amd64.tar.gz", "linux_amd64.tar.gz", "linux_arm64.tar.gz"].map((s) => `tmaegis_${v}_${s}`);
  return {
    tag_name: tag,
    name: `TMAegis ${tag}`,
    body: "## Changelog\n* fix",
    published_at: "2026-10-20T10:00:00Z",
    html_url: `https://github.com/tmaegis/tmaegis/releases/tag/${tag}`,
    prerelease: false,
    draft: false,
    assets: [...names, "checksums.txt"].map((n) => ({ name: n, size: 1000, browser_download_url: `${base}/${tag}/${n}` })),
    ...extra,
  };
}

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

/** A fetch double: the releases API and the checksums download. */
function fakeFetch(opts: { list?: unknown; status?: number; checksums?: string | null; throwApi?: boolean }) {
  const calls: string[] = [];
  const f = (async (url: string | URL | Request) => {
    const u = String(url);
    calls.push(u);
    if (u.startsWith("https://api.github.com/")) {
      if (opts.throwApi) throw new TypeError("network down");
      return json(opts.list ?? [], opts.status ?? 200);
    }
    if (u.endsWith("checksums.txt")) {
      if (opts.checksums === null) return new Response("nope", { status: 500 });
      return new Response(opts.checksums ?? "", { status: 200 });
    }
    return new Response("?", { status: 404 });
  }) as typeof fetch;
  return { f, calls };
}

describe("fallback release", () => {
  const fb = fallbackRelease();
  it("describes v0.1.0 with GitHub release URLs for the six stable and six experimental builds, never /public", () => {
    expect(fb.tag).toBe("v0.1.0");
    expect(fb.version).toBe("0.1.0");
    const archives = fb.assets.filter((a) => a.kind === "archive");
    expect(archives).toHaveLength(12);
    for (const e of EXPERIMENTAL) expect(findPackage(fb.assets, e.os, e.arch)?.kind, `${e.os}/${e.arch}`).toBe("archive");
    for (const a of fb.assets) {
      expect(isTrustedDownloadUrl(a.url), a.url).toBe(true);
      expect(a.url.startsWith(`${base}/v0.1.0/`), a.url).toBe(true);
      expect(a.url).not.toContain("/releases/tmaegis_");
    }
    for (const s of SUPPORTED) expect(findPackage(fb.assets, s.os, s.arch), `${s.os}/${s.arch}`).not.toBeNull();
    expect(fb.assets.some((a) => a.kind === "checksums")).toBe(true);
  });
  it("lists the planned Linux packages for both architectures", () => {
    for (const fmt of ["deb", "rpm"] as const) for (const arch of ["x64", "arm64"] as const) expect(findLinuxPackage(fb.assets, fmt, arch)?.name, `${fmt}/${arch}`).toBe(`tmaegis_0.1.0_linux_${arch === "x64" ? "amd64" : "arm64"}.${fmt}`);
  });
  it("does not invent sizes or hashes", () => {
    expect(fb.assets.every((a) => a.sha256 === null && a.size === null)).toBe(true);
  });
});

describe("loadReleaseData", () => {
  it("uses the latest GitHub release with sha256 from checksums.txt", async () => {
    const sums = ["windows_amd64.zip", "linux_amd64.tar.gz"].map((s, i) => `${H(String(i + 1))}  tmaegis_0.1.1_${s}`).join("\n");
    const { f, calls } = fakeFetch({ list: [release("v0.1.1"), release("v0.1.0")], checksums: sums });
    const d = await loadReleaseData({ fetchImpl: f, repo: "tmaegis/tmaegis" });
    expect(d.source).toBe("github");
    expect(d.latest?.tag).toBe("v0.1.1");
    expect(d.latest?.assets.find((a) => a.name === "tmaegis_0.1.1_windows_amd64.zip")).toMatchObject({ sha256: H("1"), size: 1000 });
    expect(d.latest?.assets.find((a) => a.name === "tmaegis_0.1.1_darwin_arm64.tar.gz")?.sha256).toBeNull();
    expect(d.previous.map((p) => p.tag)).toEqual(["v0.1.0"]);
    expect(calls[0]).toContain("/repos/tmaegis/tmaegis/releases");
  });

  it("sends the token when one is configured, and works without", async () => {
    let auth: string | null = "unset";
    const f = (async (url: string | URL | Request, init?: RequestInit) => {
      if (String(url).startsWith("https://api.github.com/")) auth = new Headers(init?.headers).get("authorization");
      return json([release("v0.1.0")]);
    }) as typeof fetch;
    await loadReleaseData({ fetchImpl: f, token: "t0k" });
    expect(auth).toBe("Bearer t0k");
    await loadReleaseData({ fetchImpl: f, token: "" });
    expect(auth).toBeNull();
  });

  it("skips prereleases and drafts and caps the list of previous versions", async () => {
    const list = [release("v0.3.0-rc1", { prerelease: true }), release("v0.2.9", { draft: true }), release("v0.2.0"), ...Array.from({ length: 14 }, (_, i) => release(`v0.1.${i}`))];
    const d = await loadReleaseData({ fetchImpl: fakeFetch({ list, checksums: "" }).f });
    expect(d.latest?.tag).toBe("v0.2.0");
    expect(d.previous).toHaveLength(MAX_PREVIOUS);
    expect(d.previous.every((p) => !p.tag.includes("rc"))).toBe(true);
  });

  it("keeps the release with null hashes when checksums.txt cannot be read", async () => {
    const d = await loadReleaseData({ fetchImpl: fakeFetch({ list: [release("v0.1.0")], checksums: null }).f });
    expect(d.source).toBe("github");
    expect(d.latest?.assets.every((a) => a.sha256 === null)).toBe(true);
  });

  it("falls back to the bundled v0.1.0 when the network is down", async () => {
    const d = await loadReleaseData({ fetchImpl: fakeFetch({ throwApi: true }).f });
    expect(d.source).toBe("fallback");
    expect(d.latest?.tag).toBe("v0.1.0");
    expect(d.previous).toEqual([]);
  });

  it.each([403, 429, 500, 502])("falls back on HTTP %i (rate limit or outage)", async (status) => {
    const d = await loadReleaseData({ fetchImpl: fakeFetch({ status, list: { message: "x" } }).f });
    expect(d.source).toBe("fallback");
  });

  it("falls back on a malformed response", async () => {
    expect((await loadReleaseData({ fetchImpl: fakeFetch({ list: { not: "an array" } }).f })).source).toBe("fallback");
  });

  it("shows 'first release coming' when GitHub has no release", async () => {
    for (const r of [fakeFetch({ list: [] }), fakeFetch({ status: 404, list: { message: "Not Found" } }), fakeFetch({ list: [release("v0.1.0", { draft: true }), release("v0.1.1", { prerelease: true })] })]) {
      const d = await loadReleaseData({ fetchImpl: r.f });
      expect(d).toEqual({ source: "none", latest: null, previous: [], image: null });
    }
  });

  it("uses the fallback while a new release has no archives uploaded yet", async () => {
    const empty = release("v0.1.1", { assets: [] });
    expect((await loadReleaseData({ fetchImpl: fakeFetch({ list: [empty] }).f })).source).toBe("fallback");
  });

  it("never offers a file that is not on GitHub releases", async () => {
    const r = release("v0.1.1");
    r.assets.push({ name: "tmaegis_0.1.1_linux_amd64.deb", size: 1, browser_download_url: "https://evil.example/x.deb" });
    const d = await loadReleaseData({ fetchImpl: fakeFetch({ list: [r], checksums: "" }).f });
    expect(d.latest?.assets.every((a) => isTrustedDownloadUrl(a.url))).toBe(true);
  });
});

describe("checkImage", () => {
  const registry = (manifest: number, token: number | "throw" = 200) =>
    (async (url: string | URL | Request, init?: RequestInit) => {
      const u = String(url);
      if (u.startsWith("https://ghcr.io/token")) {
        if (token === "throw") throw new TypeError("offline");
        return json({ token: "anon" }, token);
      }
      if (u.startsWith("https://ghcr.io/v2/tmaegis/tmaegis/manifests/0.1.0")) {
        expect(new Headers(init?.headers).get("authorization")).toBe("Bearer anon");
        return new Response("{}", { status: manifest });
      }
      return new Response("?", { status: 500 });
    }) as typeof fetch;

  it("reports a published, missing or unknown image", async () => {
    expect(await checkImage(registry(200), "TMAegis/tmaegis", "0.1.0")).toBe("published");
    expect(await checkImage(registry(404), "tmaegis/tmaegis", "0.1.0")).toBe("missing");
    expect(await checkImage(registry(401), "tmaegis/tmaegis", "0.1.0")).toBe("missing");
    expect(await checkImage(registry(503), "tmaegis/tmaegis", "0.1.0")).toBe("unknown");
    expect(await checkImage(registry(200, 500), "tmaegis/tmaegis", "0.1.0")).toBe("unknown");
    expect(await checkImage(registry(200, "throw"), "tmaegis/tmaegis", "0.1.0")).toBe("unknown");
  });

  it("is part of the release data, and unknown in the fallback", async () => {
    const live = await loadReleaseData({ fetchImpl: fakeFetch({ list: [release("v0.1.0")], checksums: "" }).f, repo: "tmaegis/tmaegis" });
    expect(live.image).toEqual({ name: "ghcr.io/tmaegis/tmaegis", ref: "ghcr.io/tmaegis/tmaegis:0.1.0", status: "unknown" });
    const fb = await loadReleaseData({ fetchImpl: fakeFetch({ throwApi: true }).f, repo: "tmaegis/tmaegis" });
    expect(fb.image?.status).toBe("unknown");
  });
});
