import { describe, expect, it } from "vitest";
import { LOCALES, REVIEWED, isBeta } from "./locales";

describe("Beta labels", () => {
  it("only en-US and en-GB are reviewed (mirrors internal/i18n/status.go)", () => {
    expect([...REVIEWED].sort()).toEqual(["en-GB", "en-US"]);
    expect(LOCALES.filter((l) => !isBeta(l.tag)).map((l) => l.tag)).toEqual(["en-US", "en-GB"]);
    expect(LOCALES.filter((l) => isBeta(l.tag))).toHaveLength(LOCALES.length - 2);
  });
});
