import path from "node:path";
import { SECURITY_HEADERS } from "./lib/security-headers.mjs";

const prod = process.env.NODE_ENV === "production";
const i18nRequestConfig = "./i18n/request.ts";

/**
 * Hybrid site: static pages for content, ISR for rule pages and downloads, server routes for search,
 * contact and health. No `output: "export"`.
 */
const nextConfig = {
  // Lets a second build live next to the default one (used to screenshot the page against a mocked GitHub API).
  distDir: process.env.NEXT_DIST_DIR || ".next",
  reactStrictMode: true,
  poweredByHeader: false,
  images: { formats: ["image/avif", "image/webp"] },
  turbopack: {
    resolveAlias: {
      "next-intl/config": i18nRequestConfig,
    },
  },
  webpack(config) {
    config.resolve ??= {};
    config.resolve.alias ??= {};
    config.resolve.alias["next-intl/config"] = path.resolve(config.context, i18nRequestConfig);
    return config;
  },
  // Server code reads the generated rule shards and the OG fonts at run time.
  outputFileTracingIncludes: {
    "/**": ["./public/data/**/*", "./generated/**/*"],
    "/og": ["./node_modules/geist/dist/fonts/geist-sans/Geist-Regular.ttf", "./node_modules/geist/dist/fonts/geist-sans/Geist-SemiBold.ttf"],
  },
  // CSP in development would block React refresh (eval), so headers apply to production builds only.
  async headers() {
    if (!prod) return [];
    return [
      { source: "/:path*", headers: SECURITY_HEADERS },
      { source: "/data/:path*", headers: [{ key: "Cache-Control", value: "public, max-age=3600, s-maxage=86400, stale-while-revalidate=604800" }] },
      // scripts/gen-icons.mjs puts a content hash in the sprite's file name
      { source: "/platform-icons.:hash.svg", headers: [{ key: "Cache-Control", value: "public, max-age=31536000, immutable" }] },
    ];
  },
};

export default nextConfig;
