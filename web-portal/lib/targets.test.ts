import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { CATEGORIES, GROUPS, TARGETS, categoryOf, groupTargets, scanCommand } from "./targets";

interface Row { id: string; count: number; manual: number; auto: number; device?: number }

function generatedTargets(): Row[] {
  const file = path.join(import.meta.dirname, "..", "generated", "coverage.json");
  if (!fs.existsSync(file)) throw new Error("generated/coverage.json is missing: run `npm run gen` before the tests");
  return (JSON.parse(fs.readFileSync(file, "utf8")) as { rows: Row[] }).rows;
}

describe("target categories", () => {
  const rows = generatedTargets();

  it("the generated data has targets to check", () => {
    expect(rows.length).toBeGreaterThan(50);
  });

  it("every real target is categorised (add new targets to lib/targets.ts)", () => {
    const missing = rows.filter((r) => !categoryOf(r.id)).map((r) => r.id);
    expect(missing).toEqual([]);
  });

  it("every mapping uses a known category and group", () => {
    for (const [id, info] of Object.entries(TARGETS)) {
      expect(CATEGORIES, id).toContain(info.category);
      expect(GROUPS[info.category], id).toContain(info.group);
      expect(info.name.length, id).toBeGreaterThan(1);
    }
  });

  it("the page shows every category the spec asks for and drops no target", () => {
    const { categories, unmapped } = groupTargets(rows);
    expect(unmapped).toEqual([]);
    expect(categories.map((c) => c.category)).toEqual([...CATEGORIES]);
    expect(categories.reduce((n, c) => n + c.targets, 0)).toBe(rows.length);
    expect(categories.reduce((n, c) => n + c.rules, 0)).toBe(rows.reduce((n, r) => n + r.count, 0));
  });

  it("puts well-known products where people look for them", () => {
    for (const [id, cat] of [["windows", "os"], ["macos", "os"], ["aix", "os"], ["ibm-i", "os"], ["postgres", "database"], ["nginx", "server"], ["kubernetes", "containers"], ["docker", "containers"], ["mcp-server", "ai"], ["chrome", "desktop"], ["cisco-ios", "network"], ["paloalto", "network"], ["aws", "cloud"], ["m365", "cloud"], ["github", "cloud"], ["ios", "mobile"], ["intune-ios", "mobile"], ["supply-chain", "devsecops"], ["printer", "hardware"]] as const) {
      expect(categoryOf(id), id).toBe(cat);
    }
  });
});

describe("groupTargets", () => {
  it("returns unlisted targets instead of dropping them", () => {
    const { categories, unmapped } = groupTargets([{ id: "brand-new", count: 3, manual: 1, auto: 2 }, { id: "nginx", count: 2, manual: 0, auto: 2 }]);
    expect(unmapped.map((u) => u.id)).toEqual(["brand-new"]);
    expect(categories).toHaveLength(1);
  });
});

describe("scanCommand", () => {
  it("adds --device-config for device-config rules and --include-manual for manual-only targets", () => {
    expect(scanCommand({ id: "aws", count: 5, manual: 1, auto: 4 })).toBe("tmaegis scan --target aws");
    expect(scanCommand({ id: "cisco-ios", count: 5, manual: 1, auto: 4, device: 4 })).toBe("tmaegis scan --target cisco-ios --device-config <file>");
    expect(scanCommand({ id: "ios", count: 5, manual: 5, auto: 0 })).toBe("tmaegis scan --target ios --include-manual");
  });
});
