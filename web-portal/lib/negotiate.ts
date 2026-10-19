import { LOCALES } from "./locales";

const BRITISH = new Set(["GB", "AU", "NZ", "IE", "IN", "ZA", "SG", "HK"]);
const HANT_REGIONS = new Set(["TW", "HK", "MO"]);

/** Maps one Accept-Language tag to a locale slug, or null when we have nothing close. */
export function slugForTag(raw: string): string | null {
  let loc: Intl.Locale;
  try {
    loc = new Intl.Locale(raw);
  } catch {
    return null;
  }
  const lang = loc.language;
  const script = loc.script;
  const region = loc.region;
  switch (lang) {
    case "en": return region && BRITISH.has(region) ? "en-gb" : "en";
    case "zh": return script === "Hant" || (!script && region && HANT_REGIONS.has(region)) ? "zh-hant" : "zh-hans";
    case "sr": return script === "Latn" ? "sr" : "sr-cyrl";
    case "pt": return "pt-br";
    case "es": return "es";
    case "nb": case "nn": return null;
    default: return LOCALES.find((l) => l.slug === lang)?.slug ?? null;
  }
}

/** First supported language from an Accept-Language header (q-values respected), or null. */
export function negotiate(header: string | null): string | null {
  if (!header) return null;
  const items = header
    .split(",")
    .map((part) => {
      const [tag, ...params] = part.trim().split(";");
      const q = Number(params.map((p) => p.trim()).find((p) => p.startsWith("q="))?.slice(2) ?? "1");
      return { tag: tag.trim(), q: Number.isFinite(q) ? q : 0 };
    })
    .filter((i) => i.tag && i.tag !== "*" && i.q > 0)
    .sort((a, b) => b.q - a.q);
  for (const i of items) {
    const slug = slugForTag(i.tag);
    if (slug) return slug;
  }
  return null;
}
