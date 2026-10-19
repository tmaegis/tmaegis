import { NextRequest } from "next/server";
import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";
import { negotiate } from "./lib/negotiate";

const intl = createMiddleware(routing);

// Next.js 16 name for middleware. Picks the language (cookie first, then Accept-Language) and redirects to /<locale>.
export default function proxy(req: NextRequest) {
  const mapped = negotiate(req.headers.get("accept-language"));
  if (mapped) {
    const headers = new Headers(req.headers);
    headers.set("accept-language", mapped);
    return intl(new NextRequest(req, { headers }));
  }
  return intl(req);
}

export const config = {
  // Everything except API routes, Next internals, static data and files with an extension.
  matcher: ["/((?!api|og|_next|_vercel|data|.*\\..*).*)"],
};
