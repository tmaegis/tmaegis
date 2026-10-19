import { NextResponse, type NextRequest } from "next/server";
import { normalizeQuery, searchRules } from "@/lib/rules-data";

export const dynamic = "force-dynamic";

/** GET /api/rules/search?q=ssh&target=linux&severity=high&level=1&type=automated&page=1&pageSize=25 */
export function GET(req: NextRequest) {
  const raw: Record<string, string> = {};
  req.nextUrl.searchParams.forEach((v, k) => (raw[k] = v));
  const result = searchRules(normalizeQuery(raw));
  return NextResponse.json(result, {
    headers: { "Cache-Control": "public, s-maxage=300, stale-while-revalidate=3600" },
  });
}
