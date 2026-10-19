import fs from "node:fs/promises";
import path from "node:path";
import { ImageResponse } from "next/og";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";

const dir = path.join(process.cwd(), "node_modules", "geist", "dist", "fonts", "geist-sans");
let fonts: Promise<[Buffer, Buffer]> | null = null;
const loadFonts = () => (fonts ??= Promise.all([fs.readFile(path.join(dir, "Geist-Regular.ttf")), fs.readFile(path.join(dir, "Geist-SemiBold.ttf"))]));

/** /og?title=...&kicker=... renders a 1200x630 card with the shield, aurora glows and the page title. */
export async function GET(req: NextRequest) {
  const sp = req.nextUrl.searchParams;
  const title = (sp.get("title") || "TMAegis").slice(0, 90);
  const kicker = (sp.get("kicker") || "Open-source CIS hardening").slice(0, 60);
  const [regular, semibold] = await loadFonts();
  const size = title.length > 60 ? 54 : title.length > 36 ? 64 : 76;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "72px 80px",
          background: "radial-gradient(900px 500px at 10% -10%, rgba(0,242,254,0.18), transparent 60%), radial-gradient(800px 500px at 100% 0%, rgba(16,185,129,0.16), transparent 60%), #0B0F19",
          color: "#E5E7EB",
          fontFamily: "Geist",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", maxWidth: 740 }}>
          <div style={{ display: "flex", fontSize: 28, color: "#34D399", letterSpacing: 4, textTransform: "uppercase" }}>{kicker}</div>
          <div style={{ display: "flex", marginTop: 28, fontSize: size, fontWeight: 600, lineHeight: 1.1, letterSpacing: -1.5 }}>{title}</div>
          <div style={{ display: "flex", marginTop: 40, fontSize: 30, color: "#00F2FE" }}>TMAegis</div>
        </div>
        <svg width="320" height="320" viewBox="0 0 120 120">
          <polygon points="60,6 108,25 108,68 60,114 12,68 12,25" fill="#111827" stroke="#00F2FE" strokeWidth="4" strokeLinejoin="round" />
          <polygon points="24,31 37,36 60,70 60,86 37,56 37,77 24,64" fill="#10B981" />
          <polygon points="96,31 83,36 60,70 60,86 83,56 83,77 96,64" fill="#10B981" />
          <polygon points="35,24 85,24 85,36 66,36 66,90 60,99 54,90 54,36 35,36" fill="#00F2FE" />
        </svg>
      </div>
    ),
    {
      width: 1200,
      height: 630,
      fonts: [
        { name: "Geist", data: regular, weight: 400, style: "normal" },
        { name: "Geist", data: semibold, weight: 600, style: "normal" },
      ],
      headers: { "Cache-Control": "public, max-age=86400, s-maxage=604800, stale-while-revalidate=86400" },
    },
  );
}
