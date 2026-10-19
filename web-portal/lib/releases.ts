import fallbackJson from "./release-fallback.json";
import { buildAssets, containerImage, fromGitHub, imageTag, parseChecksums, summaryOf, type GitHubReleaseJson, type RawAsset, type Release, type ReleaseSummary } from "./release-assets";
import { GITHUB_REPO } from "./site";

/**
 * Where the download page gets its data:
 *  - "github":   the latest GitHub Release of the public repository (ISR, refreshed every 10 minutes)
 *  - "fallback": GitHub could not be reached (offline, rate limited, error), so the bundled v0.1.0 description is used
 *  - "none":     GitHub answered and the project has no published release yet
 */
export type ReleaseSource = "github" | "fallback" | "none";

/**
 * Whether the container image of the latest release is on GitHub Container Registry:
 * "published" (the tag answers), "missing" (the registry says there is no such image or tag, or it is not public)
 * or "unknown" (not checked, or the registry could not be reached).
 */
export type ImageStatus = "published" | "missing" | "unknown";

export interface ContainerImageInfo {
  /** `ghcr.io/<owner>/<repo>` */
  name: string;
  /** `ghcr.io/<owner>/<repo>:<version>` */
  ref: string;
  status: ImageStatus;
}

export interface ReleaseData {
  source: ReleaseSource;
  latest: Release | null;
  /** Older releases, newest first, at most MAX_PREVIOUS. */
  previous: ReleaseSummary[];
  /** The image for `latest`, or null when there is no release. */
  image: ContainerImageInfo | null;
}

export const REVALIDATE_SECONDS = 600;
export const MAX_PREVIOUS = 10;
const MAX_CHECKSUM_BYTES = 256 * 1024;

type Fetch = typeof fetch;
interface Options {
  fetchImpl?: Fetch;
  repo?: string;
  token?: string;
}

/** The bundled description of v0.1.0 with the planned file names (sizes and hashes are unknown until the release exists). */
export function fallbackRelease(): Release {
  const j = fallbackJson as { tag: string; name: string; publishedAt: string | null; url: string; prerelease: boolean; body: string; assets: RawAsset[] };
  return {
    tag: j.tag,
    version: j.tag.replace(/^v/, ""),
    name: j.name,
    publishedAt: j.publishedAt,
    url: j.url,
    prerelease: j.prerelease,
    body: j.body,
    assets: buildAssets(j.assets),
  };
}

const MANIFEST_TYPES = [
  "application/vnd.oci.image.index.v1+json",
  "application/vnd.docker.distribution.manifest.list.v2+json",
  "application/vnd.oci.image.manifest.v1+json",
  "application/vnd.docker.distribution.manifest.v2+json",
].join(", ");

/** Asks GHCR (anonymously, like `docker pull` would) whether `<repo>:<tag>` exists. Never throws. */
export async function checkImage(f: Fetch, repo: string, tag: string): Promise<ImageStatus> {
  const name = repo.toLowerCase();
  const init = { next: { revalidate: REVALIDATE_SECONDS }, signal: AbortSignal.timeout(5000) };
  try {
    const tok = await f(`https://ghcr.io/token?scope=${encodeURIComponent(`repository:${name}:pull`)}&service=ghcr.io`, init as RequestInit);
    if (!tok.ok) return "unknown";
    const body = (await tok.json()) as { token?: unknown };
    if (typeof body?.token !== "string" || !body.token) return "unknown";
    const res = await f(`https://ghcr.io/v2/${name}/manifests/${encodeURIComponent(tag)}`, { ...init, headers: { Authorization: `Bearer ${body.token}`, Accept: MANIFEST_TYPES } } as RequestInit);
    if (res.ok) return "published";
    // 404: no such tag; 401/403: no such image, or it is private, so visitors cannot pull it either.
    if (res.status === 404 || res.status === 401 || res.status === 403) return "missing";
    return "unknown";
  } catch {
    return "unknown";
  }
}

function imageInfo(repo: string, version: string, status: ImageStatus): ContainerImageInfo {
  const name = containerImage(repo);
  return { name, ref: `${name}:${imageTag(version)}`, status };
}

async function fetchChecksums(f: Fetch, raw: RawAsset[], headers: Record<string, string>): Promise<Map<string, string> | undefined> {
  const asset = raw.find((a) => a.name.toLowerCase() === "checksums.txt") ?? raw.find((a) => /checksums?|sha256sums/i.test(a.name));
  if (!asset) return undefined;
  try {
    const res = await f(asset.url, { headers: { "User-Agent": headers["User-Agent"] }, next: { revalidate: REVALIDATE_SECONDS }, signal: AbortSignal.timeout(8000) } as RequestInit);
    if (!res.ok) return undefined;
    const text = await res.text();
    if (text.length > MAX_CHECKSUM_BYTES) return undefined;
    const map = parseChecksums(text);
    return map.size ? map : undefined;
  } catch {
    return undefined;
  }
}

/** Reads the releases of the public repository. Never throws: every failure ends in "fallback" or "none". */
export async function loadReleaseData(opts: Options = {}): Promise<ReleaseData> {
  const f = opts.fetchImpl ?? fetch;
  const repo = opts.repo ?? GITHUB_REPO;
  const token = opts.token ?? process.env.GITHUB_TOKEN;
  const headers: Record<string, string> = { Accept: "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28", "User-Agent": "tmaegis-web" };
  if (token) headers.Authorization = `Bearer ${token}`;
  const fb = fallbackRelease();
  const fallback: ReleaseData = { source: "fallback", latest: fb, previous: [], image: imageInfo(repo, fb.version, "unknown") };

  let list: unknown;
  try {
    const res = await f(`https://api.github.com/repos/${repo}/releases?per_page=${MAX_PREVIOUS + 5}`, {
      headers,
      next: { revalidate: REVALIDATE_SECONDS },
      signal: AbortSignal.timeout(8000),
    } as RequestInit);
    // 404: the repository (or its releases) does not exist yet. Anything else that is not OK is an outage.
    if (res.status === 404) return { source: "none", latest: null, previous: [], image: null };
    if (!res.ok) return fallback;
    list = await res.json();
  } catch {
    return fallback;
  }
  if (!Array.isArray(list)) return fallback;

  const stable = (list as GitHubReleaseJson[])
    .map((j) => fromGitHub(j, repo))
    .filter((x): x is NonNullable<typeof x> => !!x && !x.release.prerelease);
  if (stable.length === 0) return { source: "none", latest: null, previous: [], image: null };

  const [head, ...rest] = stable;
  const [checksums, imageStatus] = await Promise.all([fetchChecksums(f, head.raw, headers), checkImage(f, repo, imageTag(head.release.version))]);
  const assets = buildAssets(head.raw, checksums);
  // A release whose archives are not uploaded yet (GoReleaser is still running) cannot be offered for download.
  if (!assets.some((a) => a.kind === "archive")) return fallback;
  return {
    source: "github",
    latest: { ...head.release, assets },
    previous: rest.slice(0, MAX_PREVIOUS).map((r) => summaryOf(r.release)),
    image: imageInfo(repo, head.release.version, imageStatus),
  };
}
