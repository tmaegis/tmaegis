import type { MetadataRoute } from "next";
import { DOC_SLUGS, STATIC_PATHS, siteUrl } from "@/lib/site";
import { LOCALES } from "@/lib/locales";

export default function sitemap(): MetadataRoute.Sitemap {
  const base = siteUrl();
  const paths = [...STATIC_PATHS, ...DOC_SLUGS.map((s) => `/docs/${s}`)];
  const lastModified = new Date();
  return paths.flatMap((p) =>
    LOCALES.map((l) => ({
      url: `${base}/${l.slug}${p}`,
      lastModified,
      changeFrequency: p === "" ? ("weekly" as const) : ("monthly" as const),
      priority: p === "" ? 1 : p === "/rules" || p === "/docs" ? 0.8 : 0.6,
      alternates: {
        languages: Object.fromEntries([...LOCALES.map((x) => [x.tag, `${base}/${x.slug}${p}`]), ["x-default", `${base}/en${p}`]]),
      },
    })),
  );
}
