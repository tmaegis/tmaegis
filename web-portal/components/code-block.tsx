import { CopyButton } from "./copy-button";
import { cn } from "@/lib/utils";

/** Dark code block with a copy button. `lang` is only a hint shown in the header. */
export function CodeBlock({ code, lang, className, copy = true }: { code: string; lang?: string; className?: string; copy?: boolean }) {
  return (
    <div dir="ltr" className={cn("not-prose overflow-hidden rounded-xl border border-white/10 bg-code text-slate-200 shadow-lg", className)}>
      <div className="flex items-center justify-between border-b border-white/10 bg-white/[0.03] px-3 py-1.5">
        <span className="font-mono text-[0.7rem] uppercase tracking-wider text-slate-400">{lang ?? "shell"}</span>
        {copy && <CopyButton text={code} />}
      </div>
      <pre className="cmd-scroll p-4 text-[0.82rem] leading-relaxed" tabIndex={0}>
        <code>{code}</code>
      </pre>
    </div>
  );
}
