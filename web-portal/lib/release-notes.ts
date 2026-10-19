/**
 * Release notes come from GitHub (markdown written by GoReleaser or by hand). They are rendered through this
 * small subset parser into plain data, so no raw HTML ever reaches the page: headings, paragraphs, bullet lists,
 * `code`, **bold** and https links only.
 */
export type Inline = { t: "text"; text: string } | { t: "code"; text: string } | { t: "strong"; text: string } | { t: "link"; text: string; href: string };
export type NoteBlock = { t: "heading"; inline: Inline[] } | { t: "p"; inline: Inline[] } | { t: "ul"; items: Inline[][] };

const MAX_BLOCKS = 80;
const MAX_LINE = 400;

export function parseInline(src: string): Inline[] {
  const out: Inline[] = [];
  const re = /`([^`]+)`|\*\*([^*]+)\*\*|\[([^\]]+)\]\((https:\/\/[^\s)]+)\)/g;
  let last = 0;
  for (const m of src.matchAll(re)) {
    if (m.index > last) out.push({ t: "text", text: src.slice(last, m.index) });
    if (m[1] !== undefined) out.push({ t: "code", text: m[1] });
    else if (m[2] !== undefined) out.push({ t: "strong", text: m[2] });
    else out.push({ t: "link", text: m[3], href: m[4] });
    last = m.index + m[0].length;
  }
  if (last < src.length) out.push({ t: "text", text: src.slice(last) });
  return out;
}

export function parseNotes(body: string): NoteBlock[] {
  const blocks: NoteBlock[] = [];
  let list: Inline[][] | null = null;
  let para: string[] = [];
  const flushPara = () => {
    if (para.length) blocks.push({ t: "p", inline: parseInline(para.join(" ")) });
    para = [];
  };
  const flushList = () => {
    if (list) blocks.push({ t: "ul", items: list });
    list = null;
  };
  for (const raw of body.replace(/\r\n?/g, "\n").split("\n")) {
    const line = raw.trim().slice(0, MAX_LINE);
    // Raw HTML (comments, tags) is dropped.
    if (!line || /^<\/?[a-z!][^>]*>$/i.test(line)) {
      flushPara();
      flushList();
      continue;
    }
    const h = line.match(/^#{1,6}\s+(.+)$/);
    const li = line.match(/^[-*+]\s+(.+)$/);
    if (h) {
      flushPara();
      flushList();
      blocks.push({ t: "heading", inline: parseInline(h[1].replace(/<[^>]*>/g, "")) });
    } else if (li) {
      flushPara();
      (list ??= []).push(parseInline(li[1].replace(/<[^>]*>/g, "")));
    } else {
      flushList();
      para.push(line.replace(/<[^>]*>/g, ""));
    }
    if (blocks.length >= MAX_BLOCKS) break;
  }
  flushPara();
  flushList();
  return blocks.slice(0, MAX_BLOCKS);
}
