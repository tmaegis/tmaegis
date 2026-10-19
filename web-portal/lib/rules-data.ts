import fs from "node:fs";
import path from "node:path";

export type Severity = "critical" | "high" | "medium" | "low";

/** One row of the compact search index: [id, title, target, severityIndex, level, manual, tags]. */
export type IndexRow = [string, string, string, number, number, 0 | 1, string];

export interface IndexFile {
  v: number;
  generatedAt: string;
  severities: Severity[];
  rows: IndexRow[];
}

export interface Rule {
  id: string;
  title: string;
  description: string;
  target: string;
  os_family?: string[];
  level: number;
  severity: Severity;
  tags?: string[];
  reference?: string;
  audit: { type: string; command?: string; check?: string; file_path?: string; pattern?: string; expected_output?: string; requires_file?: string[] };
  remediation: { type?: string; command?: string; file_path?: string; replace_line?: string; [k: string]: unknown };
}

export interface RuleQuery {
  q?: string;
  target?: string;
  severity?: string;
  level?: string;
  type?: string; // "automated" | "manual"
  page?: number;
  pageSize?: number;
}

export interface RuleHit {
  id: string;
  title: string;
  target: string;
  severity: Severity;
  level: number;
  manual: boolean;
}

export interface SearchResult {
  total: number;
  catalogTotal: number;
  page: number;
  pages: number;
  pageSize: number;
  hits: RuleHit[];
  facets: { targets: { id: string; count: number }[]; severities: Record<string, number>; levels: Record<string, number> };
}

const SEVERITIES: Severity[] = ["critical", "high", "medium", "low"];
export const PAGE_SIZE = 25;
export const MAX_PAGE_SIZE = 100;

const dataDir = () => path.join(process.cwd(), "public", "data");

let indexCache: { file: IndexFile; haystack: string[]; targets: { id: string; count: number }[]; byId: Map<string, number> } | null = null;

/** Loads the compact index once per server instance (about 700 KiB for 6,000 rules). */
export function loadIndex() {
  if (indexCache) return indexCache;
  let file: IndexFile;
  try {
    file = JSON.parse(fs.readFileSync(path.join(dataDir(), "index.json"), "utf8"));
  } catch {
    file = { v: 1, generatedAt: "", severities: SEVERITIES, rows: [] };
  }
  const counts = new Map<string, number>();
  const byId = new Map<string, number>();
  file.rows.forEach((r, i) => {
    counts.set(r[2], (counts.get(r[2]) ?? 0) + 1);
    byId.set(r[0], i);
  });
  indexCache = {
    file,
    byId,
    haystack: file.rows.map((r) => `${r[0]} ${r[1]} ${r[2]} ${r[6]}`.toLowerCase()),
    targets: [...counts.entries()].map(([id, count]) => ({ id, count })).sort((a, b) => a.id.localeCompare(b.id)),
  };
  return indexCache;
}

export function normalizeQuery(raw: Record<string, string | string[] | undefined>): Required<RuleQuery> {
  const one = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v) ?? "";
  const page = Number.parseInt(one(raw.page), 10);
  const size = Number.parseInt(one(raw.pageSize), 10);
  const type = one(raw.type);
  const severity = one(raw.severity).toLowerCase();
  const level = one(raw.level);
  return {
    q: one(raw.q).trim().slice(0, 100),
    target: one(raw.target).slice(0, 64),
    severity: SEVERITIES.includes(severity as Severity) ? severity : "",
    level: level === "1" || level === "2" ? level : "",
    type: type === "automated" || type === "manual" ? type : "",
    page: Number.isFinite(page) && page > 0 ? Math.min(page, 100000) : 1,
    pageSize: Number.isFinite(size) && size > 0 ? Math.min(size, MAX_PAGE_SIZE) : PAGE_SIZE,
  };
}

/** Server-side search + filter + pagination over the index. All search terms must match (AND). */
export function searchRules(input: RuleQuery): SearchResult {
  const { file, haystack, targets } = loadIndex();
  const q = (input.q ?? "").trim().toLowerCase();
  const terms = q ? q.split(/\s+/).filter(Boolean) : [];
  const sev = input.severity ? SEVERITIES.indexOf(input.severity as Severity) : -1;
  const level = input.level ? Number(input.level) : 0;
  const pageSize = Math.min(Math.max(input.pageSize ?? PAGE_SIZE, 1), MAX_PAGE_SIZE);

  const matched: number[] = [];
  const severities: Record<string, number> = { critical: 0, high: 0, medium: 0, low: 0 };
  const levels: Record<string, number> = { "1": 0, "2": 0 };
  const targetCounts = new Map<string, number>();
  const rows = file.rows;
  for (let i = 0; i < rows.length; i++) {
    const r = rows[i];
    if (terms.length && !terms.every((t) => haystack[i].includes(t))) continue;
    if (input.type === "manual" && r[5] !== 1) continue;
    if (input.type === "automated" && r[5] !== 0) continue;
    // Facet counts ignore their own filter, so they show what switching that filter would give.
    const tOk = !input.target || r[2] === input.target;
    const sOk = sev < 0 || r[3] === sev;
    const lOk = !level || r[4] === level;
    if (tOk && lOk) severities[SEVERITIES[r[3]]]++;
    if (tOk && sOk) levels[String(r[4])] = (levels[String(r[4])] ?? 0) + 1;
    if (sOk && lOk) targetCounts.set(r[2], (targetCounts.get(r[2]) ?? 0) + 1);
    if (tOk && sOk && lOk) matched.push(i);
  }
  const total = matched.length;
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const page = Math.min(Math.max(input.page ?? 1, 1), pages);
  const slice = matched.slice((page - 1) * pageSize, page * pageSize);
  return {
    total,
    catalogTotal: rows.length,
    page,
    pages,
    pageSize,
    hits: slice.map((i) => {
      const r = rows[i];
      return { id: r[0], title: r[1], target: r[2], severity: SEVERITIES[r[3]], level: r[4], manual: r[5] === 1 };
    }),
    facets: { targets: targets.map((t) => ({ id: t.id, count: targetCounts.get(t.id) ?? 0 })), severities, levels },
  };
}

/** Finds the target of a rule id using the index (ids are unique across the catalog). */
export function targetOf(id: string): string | null {
  const { file, byId } = loadIndex();
  const i = byId.get(id);
  return i === undefined ? null : file.rows[i][2];
}

const shardCache = new Map<string, Rule[]>();

export function loadShard(target: string): Rule[] {
  if (!/^[a-z0-9-]+$/.test(target)) return [];
  const hit = shardCache.get(target);
  if (hit) return hit;
  try {
    const rules = JSON.parse(fs.readFileSync(path.join(dataDir(), "targets", `${target}.json`), "utf8")) as Rule[];
    if (shardCache.size > 12) shardCache.delete(shardCache.keys().next().value as string);
    shardCache.set(target, rules);
    return rules;
  } catch {
    return [];
  }
}

export function getRule(id: string): Rule | null {
  const target = targetOf(id);
  if (!target) return null;
  return loadShard(target).find((r) => r.id === id) ?? null;
}

export interface CoverageRow {
  id: string;
  count: number;
  manual: number;
  auto: number;
  /** Rules that run only against an exported device configuration (`--device-config`). */
  device?: number;
  level2: number;
  category: string;
  benchmarks: string[];
  how: string;
  source: string;
}
export interface Coverage {
  generatedAt: string;
  total: number;
  targets: number;
  manual: number;
  auto: number;
  level1: number;
  level2: number;
  bySeverity: Record<string, number>;
  categories: string[];
  rows: CoverageRow[];
}

export function loadCoverage(): Coverage {
  try {
    return JSON.parse(fs.readFileSync(path.join(process.cwd(), "generated", "coverage.json"), "utf8"));
  } catch {
    return { generatedAt: "", total: 0, targets: 0, manual: 0, auto: 0, level1: 0, level2: 0, bySeverity: {}, categories: [], rows: [] };
  }
}

export interface PluginInfo {
  name: string;
  kind: string;
  description: string;
  source: string;
  official: boolean;
}
export function loadPlugins(): PluginInfo[] {
  try {
    return JSON.parse(fs.readFileSync(path.join(process.cwd(), "generated", "plugins.json"), "utf8"));
  } catch {
    return [];
  }
}
