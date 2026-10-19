import { LOCALES } from "./locales";

export const GITHUB_REPO = process.env.NEXT_PUBLIC_GITHUB_REPO || "tmaegis/tmaegis";
export const GITHUB_URL = `https://github.com/${GITHUB_REPO}`;
export const CONTACT_EMAIL = process.env.NEXT_PUBLIC_CONTACT_EMAIL || "contact@tmaegis.dev";

/** The public site lives at www; the apex domain tmaegis.dev redirects there (Vercel domain settings). */
export const CANONICAL_ORIGIN = "https://www.tmaegis.dev";

/** Absolute site origin for canonical URLs, sitemap and Open Graph. Set SITE_URL for a custom domain. */
export function siteUrl(): string {
  const explicit = process.env.SITE_URL || process.env.NEXT_PUBLIC_SITE_URL;
  if (explicit) return explicit.replace(/\/$/, "");
  // Production always uses the canonical www host; preview deployments use their own URL.
  if (process.env.VERCEL_ENV && process.env.VERCEL_ENV !== "production" && process.env.VERCEL_URL) {
    return `https://${process.env.VERCEL_URL}`;
  }
  return CANONICAL_ORIGIN;
}

/** Static (non-rule) paths that exist in every locale. */
export const STATIC_PATHS = [
  "",
  "/features",
  "/rules",
  "/platforms",
  "/coverage",
  "/downloads",
  "/docs",
  "/pricing",
  "/contact",
  "/legal",
] as const;

export const DOC_SLUGS = ["install", "quick-start", "cli", "rules-format", "plugins", "reports", "gui"] as const;
export type DocSlug = (typeof DOC_SLUGS)[number];

/** hreflang alternates for a locale-less path, plus x-default. */
export function alternates(path: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const l of LOCALES) out[l.tag] = `/${l.slug}${path}`;
  out["x-default"] = `/en${path}`;
  return out;
}
