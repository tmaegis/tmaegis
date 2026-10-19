"use client";

import { useState, type FormEvent } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input, Select, Textarea } from "@/components/ui/input";
import { MESSAGE_MAX, MESSAGE_MIN, TOPICS } from "@/lib/contact";

type State = { kind: "idle" } | { kind: "sending" } | { kind: "ok" } | { kind: "error"; message: string; fields: string[] };

export function ContactForm({ defaultTopic }: { defaultTopic?: string }) {
  const t = useTranslations("contact.form");
  const [state, setState] = useState<State>({ kind: "idle" });

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = Object.fromEntries(new FormData(form).entries());
    setState({ kind: "sending" });
    try {
      const res = await fetch("/api/contact", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(data) });
      if (res.ok) {
        form.reset();
        setState({ kind: "ok" });
        return;
      }
      const j = (await res.json().catch(() => ({}))) as { error?: string; fields?: string[] };
      if (res.status === 422 && j.fields?.length) {
        setState({ kind: "error", fields: j.fields, message: j.fields.map((f) => t(`errors.${f}`, { min: MESSAGE_MIN })).join(" ") });
      } else if (res.status === 429) setState({ kind: "error", fields: [], message: t("errors.rateLimit") });
      else if (res.status === 503 || res.status === 502) setState({ kind: "error", fields: [], message: t("errors.unavailable") });
      else setState({ kind: "error", fields: [], message: t("errors.generic") });
    } catch {
      setState({ kind: "error", fields: [], message: t("errors.generic") });
    }
  }

  const bad = (f: string) => (state.kind === "error" && state.fields.includes(f) ? true : undefined);
  const label = "mb-1.5 block text-sm font-medium";

  return (
    <form onSubmit={onSubmit} className="glass grid gap-5 p-6" noValidate={false} aria-describedby="contact-privacy">
      <div className="grid gap-5 sm:grid-cols-2">
        <div>
          <label htmlFor="c-name" className={label}>{t("name")}</label>
          <Input id="c-name" name="name" required maxLength={120} autoComplete="name" aria-invalid={bad("name")} />
        </div>
        <div>
          <label htmlFor="c-email" className={label}>{t("email")}</label>
          <Input id="c-email" name="email" type="email" required maxLength={200} autoComplete="email" aria-invalid={bad("email")} />
        </div>
      </div>
      <div>
        <label htmlFor="c-topic" className={label}>{t("topic")}</label>
        <Select id="c-topic" name="topic" required defaultValue={defaultTopic && (TOPICS as readonly string[]).includes(defaultTopic) ? defaultTopic : "general"} aria-invalid={bad("topic")}>
          {TOPICS.map((k) => <option key={k} value={k}>{t(`topics.${k}`)}</option>)}
        </Select>
      </div>
      <div>
        <label htmlFor="c-message" className={label}>{t("message")}</label>
        <Textarea id="c-message" name="message" required minLength={MESSAGE_MIN} maxLength={MESSAGE_MAX} rows={7} aria-invalid={bad("message")} aria-describedby="c-hint" />
        <p id="c-hint" className="mt-1 text-xs text-muted-foreground">{t("messageHint", { max: MESSAGE_MAX })}</p>
      </div>
      {/* Honeypot: hidden from people and assistive technology, bots fill it in. */}
      <div aria-hidden className="absolute -start-[9999px] h-0 w-0 overflow-hidden">
        <label>{t("website")}<input type="text" name="website" tabIndex={-1} autoComplete="off" /></label>
      </div>
      <div className="flex flex-wrap items-center gap-4">
        <Button type="submit" size="lg" disabled={state.kind === "sending"}>{state.kind === "sending" ? t("sending") : t("submit")}</Button>
        <div role="status" aria-live="polite" className="text-sm">
          {state.kind === "ok" && <p className="text-success">{t("success")}</p>}
          {state.kind === "error" && <p className="text-destructive" role="alert">{state.message}</p>}
        </div>
      </div>
      <p id="contact-privacy" className="text-xs text-muted-foreground">{t("privacy")}</p>
    </form>
  );
}
