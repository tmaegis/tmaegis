/** The 16 supported locales: BCP 47 tag, URL slug, endonym (same as internal/i18n Names) and direction. */
export interface LocaleInfo {
  tag: string;
  slug: string;
  name: string;
  dir: "ltr" | "rtl";
  /** Script family for font selection. */
  script: "latin" | "cyrillic" | "hans" | "hant" | "ja" | "ko";
}

export const LOCALES: readonly LocaleInfo[] = [
  { tag: "en-US", slug: "en", name: "English (US)", dir: "ltr", script: "latin" },
  { tag: "en-GB", slug: "en-gb", name: "English (UK)", dir: "ltr", script: "latin" },
  { tag: "vi", slug: "vi", name: "Tiếng Việt", dir: "ltr", script: "latin" },
  { tag: "zh-Hans", slug: "zh-hans", name: "简体中文", dir: "ltr", script: "hans" },
  { tag: "zh-Hant", slug: "zh-hant", name: "繁體中文", dir: "ltr", script: "hant" },
  { tag: "es-419", slug: "es", name: "Español (Latinoamérica)", dir: "ltr", script: "latin" },
  { tag: "fr", slug: "fr", name: "Français", dir: "ltr", script: "latin" },
  { tag: "de", slug: "de", name: "Deutsch", dir: "ltr", script: "latin" },
  { tag: "ja", slug: "ja", name: "日本語", dir: "ltr", script: "ja" },
  { tag: "ko", slug: "ko", name: "한국어", dir: "ltr", script: "ko" },
  { tag: "ru", slug: "ru", name: "Русский", dir: "ltr", script: "cyrillic" },
  { tag: "tr", slug: "tr", name: "Türkçe", dir: "ltr", script: "latin" },
  { tag: "pt-BR", slug: "pt-br", name: "Português (Brasil)", dir: "ltr", script: "latin" },
  { tag: "id", slug: "id", name: "Bahasa Indonesia", dir: "ltr", script: "latin" },
  { tag: "sr-Latn", slug: "sr", name: "Srpski (latinica)", dir: "ltr", script: "latin" },
  { tag: "sr-Cyrl", slug: "sr-cyrl", name: "Српски (ћирилица)", dir: "ltr", script: "cyrillic" },
] as const;

/**
 * Locales whose catalogs a native speaker has reviewed. Mirrors `Reviewed` in internal/i18n/status.go
 * (scripts/i18n-check.mjs fails when the two differ). Every other locale is machine translated and
 * is labelled "Beta" in the language picker until it is added here.
 */
export const REVIEWED: ReadonlySet<string> = new Set(["en-US", "en-GB"]);

export function isBeta(tag: string): boolean {
  return !REVIEWED.has(tag);
}

export const DEFAULT_SLUG = "en";
export const SLUGS = LOCALES.map((l) => l.slug);

const bySlug = new Map(LOCALES.map((l) => [l.slug, l]));

export function localeInfo(slug: string): LocaleInfo {
  return bySlug.get(slug) ?? LOCALES[0];
}

/** Locales whose scripts Geist does not cover; Open Graph images use English text for them. */
export function isCjk(slug: string): boolean {
  return ["hans", "hant", "ja", "ko"].includes(localeInfo(slug).script);
}
