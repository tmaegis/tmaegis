import fs from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CANONICAL_ORIGIN, GITHUB_URL, STATIC_PATHS, alternates, siteUrl } from "./site";
import { LOCALES } from "./locales";
import robots from "../app/robots";
import sitemap from "../app/sitemap";

afterEach(() => vi.unstubAllEnvs());

describe("canonical domain", () => {
  it("is https://www.tmaegis.dev and the public repository is tmaegis/tmaegis", () => {
    expect(CANONICAL_ORIGIN).toBe("https://www.tmaegis.dev");
    expect(GITHUB_URL).toBe("https://github.com/tmaegis/tmaegis");
  });
  it("is the default site URL in production and when nothing is configured", () => {
    vi.stubEnv("SITE_URL", "");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    vi.stubEnv("VERCEL_ENV", "production");
    expect(siteUrl()).toBe(CANONICAL_ORIGIN);
  });
  it("sitemap and robots use only the canonical host", () => {
    vi.stubEnv("SITE_URL", "");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    vi.stubEnv("VERCEL_ENV", "");
    const map = sitemap();
    expect(map.length).toBeGreaterThan(STATIC_PATHS.length * LOCALES.length - 1);
    for (const e of map) {
      expect(e.url.startsWith(`${CANONICAL_ORIGIN}/`), e.url).toBe(true);
      for (const href of Object.values(e.alternates?.languages ?? {})) expect(String(href).startsWith(`${CANONICAL_ORIGIN}/`), String(href)).toBe(true);
    }
    const r = robots();
    expect(r.sitemap).toBe(`${CANONICAL_ORIGIN}/sitemap.xml`);
    expect(r.host).toBe(CANONICAL_ORIGIN);
  });
  it("hreflang alternates cover every locale plus x-default", () => {
    const a = alternates("/downloads");
    expect(Object.keys(a)).toHaveLength(LOCALES.length + 1);
    expect(a["x-default"]).toBe("/en/downloads");
    expect(a.vi).toBe("/vi/downloads");
  });
  it("no source, message or data file uses the old spelling of the name or the old repository", () => {
    const root = path.join(import.meta.dirname, "..");
    const bad = new RegExp(["tma", "geis"].join(""), "i"); // built from parts so this file does not match itself
    const hits: string[] = [];
    const walk = (dir: string) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) walk(p);
        else if (/\.(tsx?|mjs|json|md|css|svg)$/.test(e.name) && bad.test(fs.readFileSync(p, "utf8"))) hits.push(path.relative(root, p));
      }
    };
    for (const d of ["app", "components", "lib", "messages", "i18n", "data", "scripts"]) walk(path.join(root, d));
    expect(hits).toEqual([]);
  });
});
