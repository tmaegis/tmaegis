"use client";

import { Link, usePathname } from "@/i18n/navigation";
import { cn } from "@/lib/utils";

export interface NavItem {
  href: string;
  label: string;
}

export function NavLinks({ items, className, linkClass }: { items: NavItem[]; className?: string; linkClass?: string }) {
  const pathname = usePathname();
  return (
    <ul className={className}>
      {items.map((it) => {
        const active = it.href === "/" ? pathname === "/" : pathname === it.href || pathname.startsWith(`${it.href}/`);
        return (
          <li key={it.href}>
            <Link
              href={it.href}
              aria-current={active ? "page" : undefined}
              className={cn(
                "block rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground aria-[current=page]:text-foreground aria-[current=page]:bg-muted",
                linkClass,
              )}
            >
              {it.label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
