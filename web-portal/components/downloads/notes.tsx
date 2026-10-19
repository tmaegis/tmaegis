import type { Inline, NoteBlock } from "@/lib/release-notes";

function Inlines({ items }: { items: Inline[] }) {
  return (
    <>
      {items.map((x, i) =>
        x.t === "code" ? <code key={i}>{x.text}</code> : x.t === "strong" ? <strong key={i}>{x.text}</strong> : x.t === "link" ? <a key={i} href={x.href} target="_blank" rel="noopener noreferrer">{x.text}</a> : <span key={i}>{x.text}</span>,
      )}
    </>
  );
}

/** Release notes from the safe markdown subset in lib/release-notes.ts. Nothing here injects HTML. */
export function Notes({ blocks }: { blocks: NoteBlock[] }) {
  return (
    <div className="prose-tma" lang="en">
      {blocks.map((b, i) =>
        b.t === "heading" ? (
          <h3 key={i}><Inlines items={b.inline} /></h3>
        ) : b.t === "p" ? (
          <p key={i}><Inlines items={b.inline} /></p>
        ) : (
          <ul key={i}>{b.items.map((it, j) => <li key={j}><Inlines items={it} /></li>)}</ul>
        ),
      )}
    </div>
  );
}
