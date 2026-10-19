import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { DOCS, type Block } from "./docs-content";
import { DOC_SLUGS } from "./site";

const en = JSON.parse(fs.readFileSync(path.join(import.meta.dirname, "..", "messages", "en-US.json"), "utf8")).docsBody as Record<string, Record<string, string>>;

function keysOf(blocks: Block[]): string[] {
  return blocks.flatMap((b) => {
    switch (b.t) {
      case "h2": case "h3": case "p": case "note": return [b.k];
      case "ul": case "ol": return b.keys;
      case "table": return [...b.head, ...b.rows.map((r) => r.k)];
      default: return [];
    }
  });
}

describe("documentation content", () => {
  it("has a body for every doc page", () => {
    for (const slug of DOC_SLUGS) {
      expect(DOCS[slug]?.length, slug).toBeGreaterThan(0);
      expect(en[slug], slug).toBeTruthy();
    }
  });

  it("every message key used by the structure exists in en-US, and none is left over", () => {
    for (const slug of DOC_SLUGS) {
      const used = keysOf(DOCS[slug]);
      expect(new Set(used).size, `${slug} reuses a key`).toBe(used.length);
      expect(used.filter((k) => !(k in en[slug])), slug).toEqual([]);
      expect(Object.keys(en[slug]).filter((k) => !used.includes(k)), slug).toEqual([]);
    }
  });

  it("keeps commands out of the translatable strings: code blocks and first table columns live in code", () => {
    for (const slug of DOC_SLUGS) {
      for (const b of DOCS[slug]) {
        if (b.t === "code") expect(b.code.length).toBeGreaterThan(0);
        if (b.t === "table") for (const r of b.rows) expect(r.cmd.trim().length).toBeGreaterThan(0);
      }
    }
  });
});
