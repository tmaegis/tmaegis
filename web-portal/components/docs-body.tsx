import { getTranslations } from "next-intl/server";
import { CodeBlock } from "@/components/code-block";
import { Badge } from "@/components/ui/badge";
import type { Block } from "@/lib/docs-content";
import { loadPlugins } from "@/lib/rules-data";

/** `code` spans inside a sentence. */
function Inline({ text }: { text: string }) {
  const parts = text.split(/(`[^`]+`)/g);
  return (
    <>
      {parts.map((p, i) => (p.startsWith("`") && p.endsWith("`") ? <code key={i}>{p.slice(1, -1)}</code> : <span key={i}>{p}</span>))}
    </>
  );
}

export async function DocsBody({ slug, blocks, english }: { slug: string; blocks: Block[]; english: boolean }) {
  const t = await getTranslations("docs");
  const body = await getTranslations("docsBody");
  const tx = (k: string) => String(body.raw(`${slug}.${k}`));
  return (
    <div className="prose-tma" lang={english ? "en" : undefined}>
      {blocks.map((b, i) => {
        switch (b.t) {
          case "h2":
            return <h2 key={i} id={b.id}>{tx(b.k)}</h2>;
          case "h3":
            return <h3 key={i}>{tx(b.k)}</h3>;
          case "p":
            return <p key={i}><Inline text={tx(b.k)} /></p>;
          case "code":
            return <CodeBlock key={i} code={b.code} lang={b.lang} />;
          case "ul":
            return <ul key={i}>{b.keys.map((k) => <li key={k}><Inline text={tx(k)} /></li>)}</ul>;
          case "ol":
            return <ol key={i}>{b.keys.map((k) => <li key={k}><Inline text={tx(k)} /></li>)}</ol>;
          case "note":
            return <blockquote key={i}><Inline text={tx(b.k)} /></blockquote>;
          case "table":
            return (
              <div key={i} className="overflow-x-auto">
                <table>
                  <thead><tr>{b.head.map((h) => <th key={h} scope="col">{tx(h)}</th>)}</tr></thead>
                  <tbody>{b.rows.map((r) => <tr key={r.k}><td><Inline text={r.cmd} /></td><td><Inline text={tx(r.k)} /></td></tr>)}</tbody>
                </table>
              </div>
            );
          case "plugins": {
            const list = loadPlugins();
            if (!list.length) return <p key={i}>{t("noPlugins")}</p>;
            return (
              <ul key={i} className="!list-none !pl-0 space-y-3">
                {list.map((p) => (
                  <li key={p.name} className="glass p-4">
                    <div className="flex flex-wrap items-center gap-2">
                      <code className="!border-0 !bg-transparent !p-0 text-base font-semibold">{p.name}</code>
                      <Badge variant={p.official ? "accent" : "default"}>{p.official ? t("pluginOfficial") : t("pluginCommunity")}</Badge>
                      <Badge variant="default">{t("pluginKind")}: {p.kind}</Badge>
                    </div>
                    <p className="mt-1 text-sm" lang="en">{p.description}</p>
                    <p className="mt-1 font-mono text-xs text-muted-foreground">{p.source}</p>
                  </li>
                ))}
              </ul>
            );
          }
        }
      })}
    </div>
  );
}
