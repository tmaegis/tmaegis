import fs from "node:fs";
import path from "node:path";
import { Marked } from "marked";
import { GITHUB_URL } from "./site";

const isRepoPath = (h: string) => /^[\w./-]+$/.test(h) && !h.startsWith("/");
const marked = new Marked({
  gfm: true,
  async: false,
  renderer: {
    // Source files are ours, but raw HTML is dropped anyway.
    html: () => "",
    link({ href, title, tokens }) {
      const text = this.parser.parseInline(tokens);
      const external = /^https?:/i.test(href) || isRepoPath(href);
      const safe = /^(https?:|mailto:|#)/i.test(href)
        ? href
        : isRepoPath(href)
          ? `${GITHUB_URL}/blob/main/${href}` // relative repo link, e.g. LICENSE
          : "#";
      const rel = external ? ' rel="noopener noreferrer" target="_blank"' : "";
      return `<a href="${safe.replace(/"/g, "&quot;")}"${title ? ` title="${title.replace(/"/g, "&quot;")}"` : ""}${rel}>${text}</a>`;
    },
  },
});

/** Reads a generated legal document (copied from the repo root by scripts/gen-data.mjs). */
export function readGenerated(name: string): string {
  try {
    return fs.readFileSync(path.join(process.cwd(), "generated", "legal", name), "utf8");
  } catch {
    return "";
  }
}

export function renderMarkdown(src: string): string {
  return marked.parse(src) as string;
}
