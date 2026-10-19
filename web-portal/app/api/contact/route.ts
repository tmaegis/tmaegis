import { createHash } from "node:crypto";
import { NextResponse, type NextRequest } from "next/server";
import { clean, validateContact } from "@/lib/contact";
import { rateLimit } from "@/lib/rate-limit";

export const dynamic = "force-dynamic";

const MAX_BODY = 16 * 1024;

/**
 * Contact form. Validates, drops honeypot hits silently, rate-limits per client (in memory), and forwards the message
 * to CONTACT_WEBHOOK_URL (Slack-compatible JSON). Nothing is stored and no personal data is ever logged.
 */
export async function POST(req: NextRequest) {
  const json = (body: object, status: number, extra?: HeadersInit) =>
    NextResponse.json(body, { status, headers: { "Cache-Control": "no-store", ...extra } });

  const origin = req.headers.get("origin");
  if (origin && new URL(origin).host !== req.headers.get("host")) return json({ error: "forbidden" }, 403);
  if (!(req.headers.get("content-type") ?? "").includes("application/json")) return json({ error: "unsupported" }, 415);

  const ip = (req.headers.get("x-forwarded-for") ?? "").split(",")[0].trim() || "unknown";
  // Only a hash of the address is used as a key, and only in memory.
  const key = createHash("sha256").update(ip).digest("hex").slice(0, 24);
  const rl = rateLimit(key, 3, 10 * 60 * 1000);
  if (!rl.ok) return json({ error: "rate_limit" }, 429, { "Retry-After": String(rl.retryAfter) });

  const text = await req.text();
  if (text.length > MAX_BODY) return json({ error: "too_large" }, 413);
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    return json({ error: "invalid" }, 400);
  }
  const v = validateContact(body);
  if (!v.ok) return json({ error: "validation", fields: v.fields }, 422);
  if (v.bot) return json({ ok: true }, 200); // pretend success

  const hook = process.env.CONTACT_WEBHOOK_URL;
  if (!hook) return json({ error: "unavailable" }, 503);
  try {
    const { name, email, topic, message } = v.value;
    const res = await fetch(hook, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text: `*TMAegis contact* (${topic})\nFrom: ${clean(name)} <${clean(email)}>\n\n${clean(message)}` }),
      signal: AbortSignal.timeout(8000),
    });
    if (!res.ok) return json({ error: "unavailable" }, 502);
  } catch {
    return json({ error: "unavailable" }, 502);
  }
  return json({ ok: true }, 200);
}
