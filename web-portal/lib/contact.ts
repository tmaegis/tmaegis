export const TOPICS = ["general", "commercial", "partnership", "security", "press"] as const;
export const MESSAGE_MIN = 20;
export const MESSAGE_MAX = 4000;

export interface ContactInput {
  name: string;
  email: string;
  topic: (typeof TOPICS)[number];
  message: string;
  website?: string; // honeypot, must stay empty
}

export type ContactField = "name" | "email" | "topic" | "message";

const EMAIL = /^[^\s@<>()[\]\\,;:"]+@[^\s@<>()[\]\\,;:"]+\.[^\s@<>()[\]\\,;:"]{2,}$/;

/** Validates the payload. Returns the failing fields (never echoes values back). */
export function validateContact(raw: unknown): { ok: true; value: ContactInput; bot: boolean } | { ok: false; fields: ContactField[] } {
  const o = (typeof raw === "object" && raw !== null ? raw : {}) as Record<string, unknown>;
  const s = (v: unknown) => (typeof v === "string" ? v.trim() : "");
  const value: ContactInput = {
    name: s(o.name).slice(0, 120),
    email: s(o.email).slice(0, 200),
    topic: TOPICS.includes(s(o.topic) as (typeof TOPICS)[number]) ? (s(o.topic) as ContactInput["topic"]) : ("" as ContactInput["topic"]),
    message: s(o.message),
    website: s(o.website),
  };
  const fields: ContactField[] = [];
  if (value.name.length < 1) fields.push("name");
  if (!EMAIL.test(value.email)) fields.push("email");
  if (!value.topic) fields.push("topic");
  if (value.message.length < MESSAGE_MIN || value.message.length > MESSAGE_MAX) fields.push("message");
  if (fields.length) return { ok: false, fields };
  return { ok: true, value, bot: !!value.website };
}

/** Strips control characters so values cannot break out of the webhook text. */
export function clean(s: string): string {
  return s.replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, "");
}
