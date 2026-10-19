// Generates everything the site reads at build/run time. Nothing here is committed.
//
//   public/data/index.json          compact search index (one row per rule)
//   public/data/targets/<id>.json   full rules, split per target (never one giant file)
//   generated/coverage.json         per-target counts (automated/manual/device) + benchmark names + docs/COVERAGE.md "how it audits"
//   generated/plugins.json          bundled plugins + community registry
//   generated/legal/*.md            TRADEMARK.md / NOTICE copied from the repo root
//
// The rule catalogue comes from `go run . rules export` (Go on PATH) or from the JSON file named in
// TMAEGIS_RULES_JSON (used by CI when the export already ran). CIS PDFs are never read or copied.
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const web = path.resolve(here, "..");
const root = path.resolve(web, "..");
const dataDir = path.join(web, "public", "data");
const genDir = path.join(web, "generated");

const SEVERITIES = ["critical", "high", "medium", "low"];

/** Loads the exported catalogue. */
function loadRules() {
  let file = process.env.TMAEGIS_RULES_JSON;
  if (!file) {
    file = path.join(os.tmpdir(), `tmaegis-rules-${process.pid}.json`);
    execFileSync("go", ["run", ".", "rules", "export", "-r", "rules", "-o", file], { cwd: root, stdio: "inherit" });
  }
  const rules = JSON.parse(fs.readFileSync(file, "utf8"));
  if (!process.env.TMAEGIS_RULES_JSON) fs.rmSync(file, { force: true });
  return rules;
}

/** First comment line after the shebang describes the plugin. */
export function describe(src) {
  for (const line of src.split("\n").slice(1, 6)) {
    const m = line.match(/^(?:#|""")\s*(.+)$/);
    if (m) return m[1].replace(/"""$/, "").trim();
  }
  return "";
}

/** rules/<category>/.../*.yaml -> target => category (first path segment). */
function targetCategories() {
  const out = new Map();
  const walk = (dir, cat) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) walk(p, cat ?? e.name);
      else if (/\.ya?ml$/.test(e.name)) {
        const head = fs.readFileSync(p, "utf8").slice(0, 4000);
        const m = head.match(/^\s*(?:-\s*)?target:\s*["']?([\w-]+)/m);
        if (m && cat && !out.has(m[1])) out.set(m[1], cat);
      }
    }
  };
  const dir = path.join(root, "rules");
  if (fs.existsSync(dir)) walk(dir, null);
  return out;
}

/** Parses the markdown table in docs/COVERAGE.md: target => { how, source, benchmark, category }. */
function coverageTable() {
  const out = new Map();
  const file = path.join(root, "docs", "COVERAGE.md");
  if (!fs.existsSync(file)) return out;
  for (const line of fs.readFileSync(file, "utf8").split("\n")) {
    if (!line.startsWith("|") || /^\|\s*-/.test(line) || /^\|\s*Category/i.test(line)) continue;
    const c = line.split("|").slice(1, -1).map((s) => s.trim());
    if (c.length < 6) continue;
    for (const t of [...c[2].matchAll(/`([\w-]+)`/g)].map((m) => m[1])) {
      out.set(t, { category: c[0], benchmark: c[1], how: c[4].replace(/`/g, ""), source: c[5].replace(/\*/g, "") });
    }
  }
  return out;
}

function benchmarksOf(list) {
  const count = new Map();
  for (const r of list) {
    const m = (r.reference || "").match(/^(CIS .+? Benchmark) v([\d.]+)/);
    if (m) count.set(`${m[1].replace(/^CIS /, "").replace(/ Benchmark$/, "")} v${m[2]}`, (count.get(`${m[1].replace(/^CIS /, "").replace(/ Benchmark$/, "")} v${m[2]}`) ?? 0) + 1);
  }
  return [...count.entries()].sort((a, b) => b[1] - a[1]).slice(0, 4).map(([k]) => k);
}

function main() {
  const hasPrebuilt =
    fs.existsSync(path.join(dataDir, "index.json")) &&
    fs.existsSync(path.join(genDir, "coverage.json")) &&
    fs.existsSync(path.join(genDir, "plugins.json"));
  if ((process.env.SKIP_GEN || process.env.VERCEL) && hasPrebuilt && !process.env.TMAEGIS_RULES_JSON) {
    console.log("gen-data: prebuilt catalog data present, skipping Go export");
    return;
  }
  let rules;
  try {
    rules = loadRules();
  } catch (err) {
    if (hasPrebuilt) {
      console.log("gen-data: Go runtime unavailable, using committed prebuilt catalog data");
      return;
    }
    throw err;
  }
  fs.rmSync(dataDir, { recursive: true, force: true });
  fs.mkdirSync(path.join(dataDir, "targets"), { recursive: true });
  fs.mkdirSync(path.join(genDir, "legal"), { recursive: true });

  const byTarget = new Map();
  for (const r of rules) {
    if (!byTarget.has(r.target)) byTarget.set(r.target, []);
    byTarget.get(r.target).push(r);
  }
  const sevRank = (s) => SEVERITIES.indexOf(s);
  const isManual = (r) => r.audit?.type === "manual";

  const rows = [];
  for (const list of byTarget.values()) {
    list.sort((a, b) => sevRank(a.severity) - sevRank(b.severity) || a.id.localeCompare(b.id, "en", { numeric: true }));
  }
  const targetsSorted = [...byTarget.keys()].sort();
  for (const t of targetsSorted) {
    const list = byTarget.get(t);
    fs.writeFileSync(path.join(dataDir, "targets", `${t}.json`), JSON.stringify(list));
    for (const r of list) {
      rows.push([r.id, r.title, t, Math.max(0, sevRank(r.severity)), r.level, isManual(r) ? 1 : 0, (r.tags ?? []).join(" ")]);
    }
  }
  const generatedAt = new Date().toISOString();
  fs.writeFileSync(path.join(dataDir, "index.json"), JSON.stringify({ v: 1, generatedAt, severities: SEVERITIES, rows }));

  const cats = targetCategories();
  const table = coverageTable();
  const targets = targetsSorted.map((t) => {
    const list = byTarget.get(t);
    const row = table.get(t);
    const manual = list.filter(isManual).length;
    return {
      id: t,
      count: list.length,
      manual,
      auto: list.length - manual,
      // Rules that only run against an exported device configuration (--device-config).
      device: list.filter((r) => r.audit?.type === "config_regex").length,
      level2: list.filter((r) => r.level === 2).length,
      category: cats.get(t) ?? "other",
      benchmarks: benchmarksOf(list),
      how: row?.how ?? "",
      source: row?.source ?? "",
    };
  });
  const sum = (f) => rules.filter(f).length;
  const coverage = {
    generatedAt,
    total: rules.length,
    targets: targets.length,
    manual: sum(isManual),
    auto: sum((r) => !isManual(r)),
    level1: sum((r) => r.level === 1),
    level2: sum((r) => r.level === 2),
    bySeverity: Object.fromEntries(SEVERITIES.map((s) => [s, sum((r) => r.severity === s)])),
    categories: [...new Set(targets.map((t) => t.category))].sort(),
    rows: targets,
  };
  fs.writeFileSync(path.join(genDir, "coverage.json"), JSON.stringify(coverage));

  // plugins
  const dir = path.join(root, "plugins");
  const bundled = fs.existsSync(dir)
    ? fs
        .readdirSync(dir)
        .filter((f) => f.startsWith("tmaegis-plugin-"))
        .map((f) => ({
          name: f.replace(/^tmaegis-plugin-/, "").replace(/\.(sh|py|exe)$/, ""),
          kind: "executable",
          description: describe(fs.readFileSync(path.join(dir, f), "utf8")),
          source: `plugins/${f}`,
          official: true,
        }))
    : [];
  const community = JSON.parse(fs.readFileSync(path.join(web, "data", "registry.json"), "utf8"));
  fs.writeFileSync(path.join(genDir, "plugins.json"), JSON.stringify([...bundled, ...community]));

  // legal documents are rendered at build time from the repo root files
  for (const f of ["TRADEMARK.md", "NOTICE"]) {
    const src = path.join(root, f);
    fs.writeFileSync(path.join(genDir, "legal", f.endsWith(".md") ? f : `${f}.txt`), fs.existsSync(src) ? fs.readFileSync(src, "utf8") : "");
  }

  const size = fs.readdirSync(path.join(dataDir, "targets")).reduce((n, f) => n + fs.statSync(path.join(dataDir, "targets", f)).size, 0);
  console.log(
    `gen-data: ${rules.length} rules, ${targets.length} targets, index ${(fs.statSync(path.join(dataDir, "index.json")).size / 1024).toFixed(0)} KiB, shards ${(size / 1048576).toFixed(1)} MiB, ${bundled.length}+${community.length} plugins`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
