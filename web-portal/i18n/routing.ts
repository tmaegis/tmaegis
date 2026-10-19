import { defineRouting } from "next-intl/routing";
import { DEFAULT_SLUG, SLUGS } from "@/lib/locales";

export const routing = defineRouting({
  locales: SLUGS,
  defaultLocale: DEFAULT_SLUG,
  localePrefix: "always",
  // The only cookie the site sets: the visitor's language choice.
  localeCookie: { name: "NEXT_LOCALE", maxAge: 60 * 60 * 24 * 365, sameSite: "lax", secure: process.env.NODE_ENV === "production" },
  alternateLinks: false,
});
