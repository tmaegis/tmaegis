import { NextResponse } from "next/server";
import { loadIndex } from "@/lib/rules-data";

export const dynamic = "force-dynamic";

export function GET() {
  const { file } = loadIndex();
  return NextResponse.json(
    { status: "ok", rules: file.rows.length, generatedAt: file.generatedAt || null },
    { headers: { "Cache-Control": "no-store" } },
  );
}
