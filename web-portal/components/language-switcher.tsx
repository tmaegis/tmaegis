"use client";

import { useEffect, useRef } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Check, Languages } from "lucide-react";
import { Link, usePathname } from "@/i18n/navigation";
import { LOCALES, isBeta, localeInfo } from "@/lib/locales";

/** Dropdown of all 16 locales (endonyms). Plain links, so it works without JavaScript. */
export function LanguageSwitcher() {
  const t = useTranslations("lang");
  const locale = useLocale();
  const pathname = usePathname();
  const ref = useRef<HTMLDetailsElement>(null);
  const current = localeInfo(locale);

  useEffect(() => {
    const close = (e: MouseEvent | KeyboardEvent) => {
      const el = ref.current;
      if (!el?.open) return;
      if (e instanceof KeyboardEvent) {
        if (e.key === "Escape") {
          el.open = false;
          el.querySelector("summary")?.focus();
        }
      } else if (!el.contains(e.target as Node)) el.open = false;
    };
    document.addEventListener("click", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("click", close);
      document.removeEventListener("keydown", close);
    };
  }, []);

  return (
    <details ref={ref} className="relative">
      <summary
        className="inline-flex h-10 cursor-pointer items-center gap-2 whitespace-nowrap rounded-lg border border-border bg-card px-3 text-sm font-medium text-foreground hover:border-primary"
        aria-label={`${t("label")}: ${current.name}${isBeta(current.tag) ? ` · ${t("beta")}` : ""}`}
      >
        <Languages className="h-4 w-4" aria-hidden />
        {/* the full name only where the header has room for it next to the navigation */}
        <span className="hidden sm:inline lg:hidden xl:inline">{current.name}</span>
        <span className="uppercase sm:hidden lg:inline xl:hidden">{current.slug.split("-")[0]}</span>
      </summary>
      <div className="glass absolute end-0 z-50 mt-2 max-h-[70vh] w-72 max-w-[calc(100vw-2rem)] overflow-y-auto bg-card-solid p-1.5 shadow-2xl" style={{ background: "var(--card-solid)" }}>
        <ul className="grid gap-0.5">
          {LOCALES.map((l) => (
            <li key={l.tag}>
              <Link
                href={pathname}
                locale={l.slug}
                prefetch={false}
                hrefLang={l.tag}
                lang={l.tag}
                aria-current={l.slug === locale ? "true" : undefined}
                className="flex items-center justify-between rounded-lg px-3 py-2 text-sm hover:bg-muted aria-[current=true]:bg-muted aria-[current=true]:font-semibold"
              >
                <span>{l.name}{isBeta(l.tag) && <span className="text-muted-foreground"> · {t("beta")}</span>}</span>
                {l.slug === locale && <Check className="h-4 w-4 text-primary" aria-hidden />}
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </details>
  );
}
