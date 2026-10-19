import type { CSSProperties } from "react";
import {
  AppWindow,
  Bot,
  Cloud,
  Code,
  Container,
  Database,
  HardDrive,
  KeyRound,
  LayoutGrid,
  Link2,
  Monitor,
  Network,
  Printer,
  Server,
  ShieldCheck,
  Smartphone,
  Sun,
  Terminal,
  Workflow,
  type LucideIcon,
} from "lucide-react";
import { BRAND_ICONS, PLATFORM_ICONS, SPRITE_URL, type GlyphName, type IconRef } from "@/lib/platform-icons";
import type { CategoryId } from "@/lib/targets";
import { cn } from "@/lib/utils";

/**
 * Platform logos, monograms and glyphs. Always decorative (aria-hidden): the product name is the visible text
 * next to the icon. Full-colour logos keep their own colours, on a contrasting chip in a theme where they would
 * vanish (`chip`). Single-colour marks are monochrome (currentColor); their brand colour shows on hover of a
 * `.group` parent, for a selected tab or pressed chip, or with `active`, and only in a theme where it has 3:1
 * contrast (see scripts/gen-icons.mjs and .picon in app/globals.css).
 *
 * Works in server and client components (no hooks).
 */

const GLYPHS: Record<GlyphName, LucideIcon> = {
  windows: LayoutGrid,
  sun: Sun,
  key: KeyRound,
  shield: ShieldCheck,
  code: Code,
  link: Link2,
  printer: Printer,
  terminal: Terminal,
};

export const CATEGORY_ICONS: Record<CategoryId, LucideIcon> = {
  os: Monitor,
  server: Server,
  database: Database,
  containers: Container,
  desktop: AppWindow,
  network: Network,
  cloud: Cloud,
  mobile: Smartphone,
  devsecops: Workflow,
  ai: Bot,
  hardware: HardDrive,
};

/** The drawing for a target id or system name, or null when it has none (the caller shows a category glyph). */
export function iconRef(id: string): IconRef | null {
  return PLATFORM_ICONS[id] ?? null;
}

interface Props {
  id: string;
  /** Pixel size of the icon (or of the tile with `tile`). */
  size?: number;
  className?: string;
  /** Shows the brand colour now (a selected item), where its contrast allows. */
  active?: boolean;
  /** Draws the icon on a small rounded tile, the way lists and cards show it. */
  tile?: boolean;
  /** Category glyph to fall back to for an unknown id. */
  category?: CategoryId;
}

function Mono({ text, size, framed }: { text: string; size: number; framed: boolean }) {
  // Inside a tile the letters can be larger, without a frame of their own.
  const fs = framed ? [0, 11.5, 10, 8][text.length] ?? 7 : [0, 15, 12.5, 10][text.length] ?? 9;
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden focusable="false" className="picon-svg">
      {framed && <rect x="2.75" y="2.75" width="18.5" height="18.5" rx="5" fill="none" stroke="currentColor" strokeWidth="1.6" />}
      <text x="12" y="12.6" textAnchor="middle" dominantBaseline="middle" fontSize={fs} fontWeight="700" fill="currentColor" letterSpacing="-0.3" style={{ fontFamily: "var(--font-stack-sans)" }}>
        {text}
      </text>
    </svg>
  );
}

function Drawing({ icon: r, size, category, framed }: { icon: IconRef | null; size: number; category?: CategoryId; framed: boolean }) {
  if (r && "brand" in r) {
    return (
      // The logo lives once in public/platform-icons.svg (cached), not in every page's HTML.
      <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden focusable="false" className="picon-svg" fill="currentColor">
        <use href={`${SPRITE_URL}#${r.brand}`} />
      </svg>
    );
  }
  if (r && "mono" in r) return <Mono text={r.mono} size={size} framed={framed} />;
  const Glyph = r && "glyph" in r ? GLYPHS[r.glyph] : CATEGORY_ICONS[category ?? "os"];
  return <Glyph width={size} height={size} strokeWidth={1.75} aria-hidden focusable="false" className="picon-svg" />;
}

export function PlatformIcon({ id, size = 20, className, active, tile, category }: Props) {
  const r = iconRef(id);
  const logo = r && "brand" in r ? BRAND_ICONS[r.brand] : null;
  // A full-colour logo is never recoloured: only single-colour marks take part in the brand-colour hover.
  const b = logo && !logo.color ? logo : null;
  const style = b ? ({ ...(b.dark ? { "--bd": `#${b.hex}` } : {}), ...(b.light ? { "--bl": `#${b.hex}` } : {}) } as CSSProperties) : undefined;
  // Monogram letters need more of the tile than a logo does to stay legible; so does a wide wordmark (IBM, Cisco).
  const inner = tile ? Math.round(size * (r && "mono" in r ? 0.82 : logo?.wide ? 0.8 : 0.58)) : size;
  return (
    <span
      aria-hidden
      className={cn("picon", tile && "picon-tile", className)}
      data-b={b ? "" : undefined}
      data-chip={logo?.chip}
      data-active={active ? "" : undefined}
      style={{ ...style, ...(tile ? { width: size, height: size } : {}) }}
    >
      <Drawing icon={r} size={inner} category={category} framed={!tile} />
    </span>
  );
}

/** A category glyph (operating systems, databases, network ...), in the same tile style when `tile` is set. */
export function CategoryIcon({ id, size = 20, className, tile }: { id: CategoryId; size?: number; className?: string; tile?: boolean }) {
  const Glyph = CATEGORY_ICONS[id];
  const inner = tile ? Math.round(size * 0.55) : size;
  return (
    <span aria-hidden className={cn("picon", tile && "picon-tile picon-tile-primary", className)} style={tile ? { width: size, height: size } : undefined}>
      <Glyph width={inner} height={inner} strokeWidth={1.75} aria-hidden focusable="false" />
    </span>
  );
}
