// Consistency checks for messages/<tag>.json against the en-US source of truth:
//   - every file is valid JSON and every message is valid ICU MessageFormat
//   - no key exists that en-US does not have; no empty values
//   - the same argument names ({name}, plural/select selectors) as en-US
//   - plural blocks have `other` and only categories that locale uses
//   - en-US plurals have `one` and `other`
//   - docsBody.* (documentation bodies) are raw text, not ICU: they are checked for identical `code` spans instead,
//     so commands, flags and file names can never be translated by accident
//   - lib/locales.ts matches internal/i18n Tags/Names and the Reviewed (non-Beta) set (when the Go source is present)
// Missing keys are NOT errors: other locales fall back to en-US at load time and are filled in later.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse, TYPE } from "@formatjs/icu-messageformat-parser";

const here = path.dirname(fileURLToPath(import.meta.url));
const web = path.resolve(here, "..");
const dir = path.join(web, "messages");
const errors = [];
const err = (file, msg) => errors.push(`${file}: ${msg}`);

function flatten(obj, prefix = "", out = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === "object") flatten(v, key, out);
    else out[key] = v;
  }
  return out;
}

/** Collects { args: Map(name -> type), plurals: [{ arg, options[] }] } from a parsed message. */
function analyse(ast, info = { args: new Map(), plurals: [] }) {
  for (const el of ast) {
    switch (el.type) {
      case TYPE.argument: info.args.set(el.value, info.args.get(el.value) ?? "arg"); break;
      case TYPE.number: case TYPE.date: case TYPE.time: info.args.set(el.value, "fmt"); break;
      case TYPE.plural:
        info.args.set(el.value, "plural");
        info.plurals.push({ arg: el.value, options: Object.keys(el.options) });
        for (const o of Object.values(el.options)) analyse(o.value, info);
        break;
      case TYPE.select:
        info.args.set(el.value, "select");
        for (const o of Object.values(el.options)) analyse(o.value, info);
        break;
      case TYPE.tag: analyse(el.children, info); break;
    }
  }
  return info;
}

function load(file) {
  try {
    return JSON.parse(fs.readFileSync(path.join(dir, file), "utf8"));
  } catch (e) {
    err(file, `invalid JSON: ${e.message}`);
    return null;
  }
}

const files = fs.readdirSync(dir).filter((f) => f.endsWith(".json")).sort();
const enRaw = load("en-US.json");
if (!enRaw) { console.error(errors.join("\n")); process.exit(1); }
const en = flatten(enRaw);
const enInfo = {};
const isRaw = (k) => k.startsWith("docsBody.");
/** The `inline code` spans of a raw documentation string, sorted. */
const codeSpans = (v) => [...v.matchAll(/`([^`]*)`/g)].map((m) => m[1]).sort();
for (const [k, v] of Object.entries(en)) {
  if (typeof v !== "string" || !v.trim()) { err("en-US.json", `${k}: empty or non-string`); continue; }
  if (isRaw(k)) {
    if ((v.match(/`/g) ?? []).length % 2) err("en-US.json", `${k}: unbalanced backticks`);
    continue;
  }
  try {
    enInfo[k] = analyse(parse(v));
    for (const p of enInfo[k].plurals) {
      for (const need of ["one", "other"]) if (!p.options.includes(need)) err("en-US.json", `${k}: plural {${p.arg}} lacks "${need}"`);
    }
  } catch (e) {
    err("en-US.json", `${k}: invalid ICU message: ${e.message}`);
  }
}

const tagOf = (f) => f.replace(/\.json$/, "");
for (const f of files) {
  if (f === "en-US.json") continue;
  const tag = tagOf(f);
  const raw = load(f);
  if (!raw) continue;
  let cats;
  try { cats = new Set(new Intl.PluralRules(tag).resolvedOptions().pluralCategories); } catch { err(f, `unknown locale tag ${tag}`); continue; }
  cats.add("other");
  for (const [k, v] of Object.entries(flatten(raw))) {
    if (!(k in en)) { err(f, `${k}: key does not exist in en-US`); continue; }
    if (typeof v !== "string" || !v.trim()) { err(f, `${k}: empty or non-string`); continue; }
    if (isRaw(k)) {
      const a = codeSpans(v).join("\u0000");
      const b = codeSpans(en[k]).join("\u0000");
      if (a !== b) err(f, `${k}: \`code\` spans must stay exactly as in en-US (commands, flags and file names are not translated)`);
      continue;
    }
    let info;
    try { info = analyse(parse(v)); } catch (e) { err(f, `${k}: invalid ICU message: ${e.message}`); continue; }
    const want = enInfo[k];
    if (!want) continue;
    const a = [...info.args.keys()].sort().join(",");
    const b = [...want.args.keys()].sort().join(",");
    if (a !== b) err(f, `${k}: placeholders {${a}} differ from en-US {${b}}`);
    for (const p of info.plurals) {
      if (!p.options.includes("other")) err(f, `${k}: plural {${p.arg}} lacks "other"`);
      for (const o of p.options) if (!o.startsWith("=") && !cats.has(o)) err(f, `${k}: plural category "${o}" is not used by ${tag}`);
    }
  }
}

// lib/locales.ts <-> internal/i18n
try {
  const go = fs.readFileSync(path.resolve(web, "..", "internal", "i18n", "i18n.go"), "utf8");
  const ts = fs.readFileSync(path.join(web, "lib", "locales.ts"), "utf8");
  const names = [...go.matchAll(/"([A-Za-z0-9-]+)":\s*"([^"]+)"/g)].map((m) => [m[1], m[2]]);
  for (const [tag, name] of names) {
    if (!ts.includes(`tag: "${tag}"`)) err("lib/locales.ts", `locale ${tag} from internal/i18n is missing`);
    else if (!ts.includes(`name: "${name}"`)) err("lib/locales.ts", `endonym for ${tag} differs from internal/i18n ("${name}")`);
  }
} catch {
  /* Go sources not available (e.g. a partial checkout): skip */
}

// Reviewed (non-Beta) locales: internal/i18n/status.go <-> REVIEWED in lib/locales.ts
try {
  const go = fs.readFileSync(path.resolve(web, "..", "internal", "i18n", "status.go"), "utf8");
  const block = go.match(/Reviewed\s*=\s*map\[string\]bool\{([\s\S]*?)\}/)?.[1] ?? "";
  const goSet = [...block.matchAll(/"([A-Za-z0-9-]+)":\s*true/g)].map((m) => m[1]).sort();
  const ts = fs.readFileSync(path.join(web, "lib", "locales.ts"), "utf8");
  const tsBlock = ts.match(/REVIEWED[^=]*=\s*new Set\(\[([^\]]*)\]\)/)?.[1] ?? "";
  const tsSet = [...tsBlock.matchAll(/"([A-Za-z0-9-]+)"/g)].map((m) => m[1]).sort();
  if (goSet.join(",") !== tsSet.join(",")) err("lib/locales.ts", `REVIEWED (${tsSet.join(", ")}) differs from internal/i18n/status.go (${goSet.join(", ")})`);
} catch {
  /* Go sources not available: skip */
}

const total = Object.keys(en).length;
if (errors.length) {
  console.error(`i18n-check failed (${errors.length}):\n  ${errors.join("\n  ")}`);
  process.exit(1);
}
console.log(`i18n-check ok: ${total} en-US keys, ${files.length} message files (${files.join(", ")})`);
