import { describe, expect, it } from "vitest";
import { loadMessages, mergeMessages } from "./messages";

describe("messages", () => {
  it("merges deeply and keeps base keys", () => {
    expect(mergeMessages({ a: { b: "1", c: "2" }, d: "x" }, { a: { b: "9" } })).toEqual({ a: { b: "9", c: "2" }, d: "x" });
  });
  it("falls back to en-US for unknown locale slugs", async () => {
    const en = await loadMessages("en");
    const fallback = await loadMessages("unknown");
    expect(fallback).toEqual(en);
  });
  it("loads translated locale catalogs over en-US", async () => {
    const en = await loadMessages("en");
    const vi = (await loadMessages("vi")) as { nav: { home: string } };
    expect(vi).not.toEqual(en);
    expect(vi.nav.home).toBeTruthy();
  });
  it("applies en-GB spelling overrides", async () => {
    const gb = (await loadMessages("en-gb")) as { footer: { license: string } };
    expect(gb.footer.license).toBe("Licence");
  });
});
