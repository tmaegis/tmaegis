import { describe, expect, it } from "vitest";
import { parseInline, parseNotes } from "./release-notes";

describe("parseNotes", () => {
  it("keeps headings, lists, paragraphs and inline formatting", () => {
    const b = parseNotes("## Changelog\n\n* abc123 Fix `scan` **now**\n* [Docs](https://github.com/tmaegis/tmaegis)\n\nPlain text\nsecond line");
    expect(b[0]).toMatchObject({ t: "heading" });
    expect(b[1]).toMatchObject({ t: "ul" });
    expect((b[1] as { items: unknown[] }).items).toHaveLength(2);
    expect(b[2]).toMatchObject({ t: "p" });
    expect(parseInline("a `b` **c** [d](https://x.dev/y)").map((x) => x.t)).toEqual(["text", "code", "text", "strong", "text", "link"]);
  });
  it("drops raw HTML, scripts and non-https links", () => {
    const b = parseNotes('<script>alert(1)</script>\n<!-- c -->\n- <img src=x onerror=alert(1)> hi\n[x](javascript:alert(1))');
    const text = JSON.stringify(b);
    expect(text).not.toContain("<script");
    expect(text).not.toContain("onerror");
    expect(b.flatMap((x) => (x.t === "p" ? x.inline : []))).not.toContainEqual(expect.objectContaining({ t: "link" }));
  });
  it("handles empty input and limits the size", () => {
    expect(parseNotes("")).toEqual([]);
    expect(parseNotes(Array.from({ length: 500 }, (_, i) => `- item ${i}\n\npara ${i}`).join("\n")).length).toBeLessThanOrEqual(80);
  });
});
