import { localeInfo } from "@/lib/locales";

type Messages = { [key: string]: string | Messages };

function isObj(v: unknown): v is Messages {
  return typeof v === "object" && v !== null;
}

/** Deep merge: values from `over` replace `base`; missing keys fall back to `base`. */
export function mergeMessages(base: Messages, over: Messages): Messages {
  const out: Messages = { ...base };
  for (const [k, v] of Object.entries(over)) {
    const b = out[k];
    out[k] = isObj(v) && isObj(b) ? mergeMessages(b, v) : (v as string | Messages);
  }
  return out;
}

async function load(tag: string): Promise<Messages | null> {
  try {
    return (await import(`../messages/${tag}.json`)).default as Messages;
  } catch {
    return null;
  }
}

/** en-US is the source of truth; every other locale is merged over it so nothing is ever missing. */
export async function loadMessages(slug: string): Promise<Messages> {
  const base = (await load("en-US")) ?? {};
  const tag = localeInfo(slug).tag;
  if (tag === "en-US") return base;
  const over = await load(tag);
  return over ? mergeMessages(base, over) : base;
}

/**
 * True when the locale's own catalog (not the en-US fallback) provides every key under `path`, for example
 * `docsBody.install`. English locales always count as translated.
 */
export async function isTranslated(slug: string, path: string): Promise<boolean> {
  const tag = localeInfo(slug).tag;
  if (tag === "en-US" || tag === "en-GB") return true;
  const own = await load(tag);
  const mine = path.split(".").reduce<unknown>((o, k) => (isObj(o) ? o[k] : undefined), own);
  const base = path.split(".").reduce<unknown>((o, k) => (isObj(o) ? o[k] : undefined), await load("en-US"));
  if (!isObj(mine) || !isObj(base)) return false;
  return Object.keys(base).every((k) => typeof mine[k] === "string" && mine[k] !== "");
}
