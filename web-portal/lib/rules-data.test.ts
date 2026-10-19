import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

const rows = [
  ["B-2", "Disable server_tokens", "nginx", 3, 1, 0, "web"],
  ["A-1", "Root MFA", "aws", 0, 1, 1, ""],
  ["C-3", "SSH root login", "linux", 0, 2, 0, "ssh"],
  ["D-4", "Firewall enabled", "linux", 1, 1, 0, ""],
];
const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tma-"));

beforeAll(() => {
  fs.mkdirSync(path.join(dir, "public", "data", "targets"), { recursive: true });
  fs.writeFileSync(path.join(dir, "public", "data", "index.json"), JSON.stringify({ v: 1, generatedAt: "x", severities: ["critical", "high", "medium", "low"], rows }));
  fs.writeFileSync(path.join(dir, "public", "data", "targets", "linux.json"), JSON.stringify([{ id: "C-3", title: "SSH root login", target: "linux", level: 2, severity: "critical", audit: { type: "command" }, remediation: {} }]));
  vi.spyOn(process, "cwd").mockReturnValue(dir);
});
afterAll(() => fs.rmSync(dir, { recursive: true, force: true }));

describe("searchRules", async () => {
  const m = await import("./rules-data");
  it("returns everything without filters, sorted as indexed", () => {
    const r = m.searchRules({});
    expect(r.total).toBe(4);
    expect(r.catalogTotal).toBe(4);
  });
  it("matches all terms against id, title, target and tags", () => {
    expect(m.searchRules({ q: "ssh root" }).hits.map((h) => h.id)).toEqual(["C-3"]);
    expect(m.searchRules({ q: "WEB" }).hits.map((h) => h.id)).toEqual(["B-2"]);
    expect(m.searchRules({ q: "root mfa linux" }).total).toBe(0);
  });
  it("combines filters", () => {
    expect(m.searchRules({ target: "linux", severity: "critical" }).hits.map((h) => h.id)).toEqual(["C-3"]);
    expect(m.searchRules({ level: "2" }).total).toBe(1);
    expect(m.searchRules({ type: "manual" }).hits.map((h) => h.id)).toEqual(["A-1"]);
    expect(m.searchRules({ type: "automated" }).total).toBe(3);
  });
  it("paginates and clamps the page", () => {
    const r = m.searchRules({ pageSize: 3, page: 2 });
    expect(r.pages).toBe(2);
    expect(r.hits).toHaveLength(1);
    expect(m.searchRules({ pageSize: 3, page: 99 }).page).toBe(2);
  });
  it("facets ignore their own filter", () => {
    const r = m.searchRules({ severity: "critical" });
    expect(r.facets.severities.critical).toBe(2);
    expect(r.facets.severities.high).toBe(1);
    expect(r.facets.targets.find((t) => t.id === "linux")?.count).toBe(1);
  });
  it("normalizes untrusted query input", () => {
    const q = m.normalizeQuery({ severity: "HIGH", level: "9", type: "x", page: "-3", pageSize: "100000", q: ["  a  ", "b"] });
    expect(q).toMatchObject({ severity: "high", level: "", type: "", page: 1, pageSize: 100, q: "a" });
  });
  it("loads a rule through its shard and rejects bad targets", () => {
    expect(m.getRule("C-3")?.title).toBe("SSH root login");
    expect(m.getRule("nope")).toBeNull();
    expect(m.loadShard("../../etc/passwd")).toEqual([]);
  });
});
