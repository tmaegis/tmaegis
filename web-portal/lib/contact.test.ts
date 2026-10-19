import { describe, expect, it } from "vitest";
import { clean, validateContact } from "./contact";
import { rateLimit, resetRateLimit } from "./rate-limit";

const ok = { name: "Ada", email: "ada@example.com", topic: "general", message: "Hello, I would like to talk about licensing." };

describe("validateContact", () => {
  it("accepts a valid message", () => {
    const r = validateContact(ok);
    expect(r.ok).toBe(true);
  });
  it("reports failing fields without echoing values", () => {
    const r = validateContact({ name: "", email: "nope", topic: "x", message: "short" });
    expect(r).toEqual({ ok: false, fields: ["name", "email", "topic", "message"] });
  });
  it("flags the honeypot", () => {
    const r = validateContact({ ...ok, website: "http://spam" });
    expect(r.ok && r.bot).toBe(true);
  });
  it("tolerates non-object input", () => {
    expect(validateContact(null).ok).toBe(false);
    expect(validateContact("x").ok).toBe(false);
  });
  it("strips control characters", () => {
    expect(clean("a\u0000b\u0007c\nd")).toBe("abc\nd");
  });
});

describe("rateLimit", () => {
  it("allows up to the limit inside the window, then resets", () => {
    resetRateLimit();
    expect(rateLimit("k", 2, 1000, 0).ok).toBe(true);
    expect(rateLimit("k", 2, 1000, 10).ok).toBe(true);
    const blocked = rateLimit("k", 2, 1000, 20);
    expect(blocked.ok).toBe(false);
    expect(blocked.retryAfter).toBeGreaterThan(0);
    expect(rateLimit("k", 2, 1000, 1500).ok).toBe(true);
    expect(rateLimit("other", 2, 1000, 20).ok).toBe(true);
  });
});
