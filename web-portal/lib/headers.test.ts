import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { SECURITY_HEADERS } from "./security-headers.mjs";
import { alternates } from "./site";
import { LOCALES, localeInfo } from "./locales";

describe("security headers", () => {
  const vercel = JSON.parse(fs.readFileSync(path.join(__dirname, "..", "vercel.json"), "utf8"));
  const global = vercel.headers.find((h: { source: string }) => h.source === "/(.*)").headers as { key: string; value: string }[];
  it("vercel.json carries exactly the headers next.config.mjs uses", () => {
    expect(Object.fromEntries(global.map((h) => [h.key, h.value]))).toEqual(Object.fromEntries(SECURITY_HEADERS.map((h) => [h.key, h.value])));
  });
  it("covers the required headers", () => {
    const keys = global.map((h) => h.key);
    for (const k of ["Content-Security-Policy", "Strict-Transport-Security", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy"]) expect(keys).toContain(k);
  });
});

describe("locales", () => {
  it("has 16 unique slugs and tags", () => {
    expect(LOCALES).toHaveLength(16);
    expect(new Set(LOCALES.map((l) => l.slug)).size).toBe(16);
    expect(new Set(LOCALES.map((l) => l.tag)).size).toBe(16);
  });
  it("maps slugs to tags and builds hreflang alternates", () => {
    expect(localeInfo("sr-cyrl").tag).toBe("sr-Cyrl");
    expect(localeInfo("unknown").tag).toBe("en-US");
    const a = alternates("/rules");
    expect(a["x-default"]).toBe("/en/rules");
    expect(a["pt-BR"]).toBe("/pt-br/rules");
    expect(Object.keys(a)).toHaveLength(17);
  });
});

import { negotiate } from "./negotiate";
describe("Accept-Language negotiation", () => {
  it("maps browser tags to site slugs", () => {
    expect(negotiate("en-US,en;q=0.9")).toBe("en");
    expect(negotiate("en-AU")).toBe("en-gb");
    expect(negotiate("zh-CN,zh;q=0.9")).toBe("zh-hans");
    expect(negotiate("zh-TW")).toBe("zh-hant");
    expect(negotiate("sr-Latn-RS")).toBe("sr");
    expect(negotiate("sr-RS")).toBe("sr-cyrl");
    expect(negotiate("pt-PT")).toBe("pt-br");
    expect(negotiate("es-MX")).toBe("es");
    expect(negotiate("xx, fr;q=0.5, de;q=0.8")).toBe("de");
    expect(negotiate("xx")).toBeNull();
    expect(negotiate(null)).toBeNull();
  });
});
