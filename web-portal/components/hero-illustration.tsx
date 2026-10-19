import type { CSSProperties } from "react";
import { BRAND_ICONS, SPRITE_URL, type BrandSlug } from "@/lib/platform-icons";

/**
 * The home page hero: the TMAegis shield with a scanning line, ringed by the platforms it hardens.
 * Inline SVG (no request), colours from the theme tokens, light CSS motion (app/globals.css .hero-*),
 * all of it stopped by prefers-reduced-motion. One element with role="img" and a translated label.
 */

const OUTER: BrandSlug[] = ["linux", "apple", "docker", "kubernetes", "postgresql", "nginx", "cisco", "l-google-cloud"];
const INNER: BrandSlug[] = ["redhat", "ubuntu", "mysql", "redis", "freebsd", "github"];

function ring(slugs: BrandSlug[], r: number, offset: number, node: number) {
  return slugs.map((slug, i) => {
    const a = ((i / slugs.length) * 360 + offset) * (Math.PI / 180);
    const cx = +(200 + r * Math.cos(a)).toFixed(2);
    const cy = +(200 + r * Math.sin(a)).toFixed(2);
    const b = BRAND_ICONS[slug];
    const style = b.color ? {} : { ...(b.dark ? { "--bd": `#${b.hex}` } : {}), ...(b.light ? { "--bl": `#${b.hex}` } : {}) } as CSSProperties;
    const s = node * 1.05;
    return (
      <g key={slug} className="hero-node picon" data-b={b.color ? undefined : ""} style={style}>
        <circle cx={cx} cy={cy} r={node} fill="var(--card-solid)" stroke="var(--border-strong)" strokeWidth="1" />
        <svg x={cx - s / 2} y={cy - s / 2} width={s} height={s} viewBox="0 0 24 24" fill="var(--pb)">
          <use href={`${SPRITE_URL}#${slug}`} />
        </svg>
      </g>
    );
  });
}

export function HeroIllustration({ label }: { label: string }) {
  return (
    <div className="hero-art" role="img" aria-label={label}>
      <svg viewBox="0 0 400 400" aria-hidden focusable="false">
        <defs>
          <linearGradient id="hero-shield-bg" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="#0B0F19" />
            <stop offset="1" stopColor="#111827" />
          </linearGradient>
          <linearGradient id="hero-scan" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="#00F2FE" stopOpacity="0" />
            <stop offset="0.75" stopColor="#00F2FE" stopOpacity="0.3" />
            <stop offset="0.97" stopColor="#FFFFFF" stopOpacity="0.85" />
            <stop offset="1" stopColor="#FFFFFF" stopOpacity="0" />
          </linearGradient>
          <clipPath id="hero-shield-clip">
            <polygon points="60,6 108,25 108,68 60,114 12,68 12,25" transform="translate(130 130) scale(1.1667)" />
          </clipPath>
        </defs>

        {/* orbits */}
        <circle cx="200" cy="200" r="168" fill="none" stroke="var(--border)" strokeWidth="1" />
        <circle cx="200" cy="200" r="118" fill="none" stroke="var(--border-strong)" strokeWidth="1" strokeDasharray="2 6" />
        <circle cx="200" cy="200" r="80" fill="none" stroke="var(--border)" strokeWidth="1" />

        {/* spokes from the shield to the outer ring, faint */}
        <g stroke="var(--primary)" strokeOpacity="0.18" strokeWidth="1">
          {[0, 45, 90, 135].map((d) => {
            const a = (d * Math.PI) / 180;
            return <line key={d} x1={200 - 168 * Math.cos(a)} y1={200 - 168 * Math.sin(a)} x2={200 + 168 * Math.cos(a)} y2={200 + 168 * Math.sin(a)} />;
          })}
        </g>

        {/* pings: checks completing */}
        <circle className="hero-ping" cx="200" cy="82" r="5" fill="none" stroke="var(--accent-text)" strokeWidth="1.5" />
        <circle className="hero-ping d2" cx="318" cy="200" r="5" fill="none" stroke="var(--accent-text)" strokeWidth="1.5" />
        <circle className="hero-ping d3" cx="116" cy="284" r="5" fill="none" stroke="var(--accent-text)" strokeWidth="1.5" />
        <circle cx="200" cy="82" r="3" fill="var(--accent-text)" />
        <circle cx="318" cy="200" r="3" fill="var(--accent-text)" />
        <circle cx="116" cy="284" r="3" fill="var(--accent-text)" />

        <g className="hero-orbit">{ring(OUTER, 168, -90, 21)}</g>
        <g className="hero-orbit rev">{ring(INNER, 118, -60, 16)}</g>

        {/* the shield (public/logo-mark.svg, scaled to 140 px) */}
        <g transform="translate(130 130) scale(1.1667)">
          <polygon points="60,6 108,25 108,68 60,114 12,68 12,25" fill="url(#hero-shield-bg)" stroke="#00F2FE" strokeWidth="5" strokeLinejoin="round" />
          <polygon points="24,31 37,36 60,70 60,86 37,56 37,77 24,64" fill="#10B981" />
          <polygon points="96,31 83,36 60,70 60,86 83,56 83,77 96,64" fill="#10B981" />
          <polygon points="35,24 85,24 85,36 66,36 66,90 60,99 54,90 54,36 35,36" fill="#00F2FE" />
        </g>
        <g clipPath="url(#hero-shield-clip)">
          <rect className="hero-scan" x="130" y="106" width="140" height="34" fill="url(#hero-scan)" />
        </g>
      </svg>
    </div>
  );
}
