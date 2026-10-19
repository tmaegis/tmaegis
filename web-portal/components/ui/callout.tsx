import type { ReactNode } from "react";
import { FlaskConical, Info, TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";

const TONES = {
  info: { box: "border-border bg-muted/40 text-muted-foreground", icon: Info, iconClass: "text-primary" },
  warning: { box: "border-warning/50 bg-warning/10 text-foreground", icon: TriangleAlert, iconClass: "text-warning" },
  experimental: { box: "border-warning/50 bg-warning/10 text-foreground", icon: FlaskConical, iconClass: "text-warning" },
} as const;

/** A note, warning or experimental notice: same border, radius, padding and icon placement on every page. */
export function Callout({ tone = "info", title, children, className, role = "note" }: { tone?: keyof typeof TONES; title?: ReactNode; children?: ReactNode; className?: string; role?: string }) {
  const { box, icon: Icon, iconClass } = TONES[tone];
  return (
    <div role={role} className={cn("flex items-start gap-2.5 rounded-lg border p-3 text-sm", box, className)}>
      <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", iconClass)} aria-hidden />
      <div className="min-w-0 space-y-1">
        {title && <p className="font-semibold text-foreground">{title}</p>}
        {children && <div>{children}</div>}
      </div>
    </div>
  );
}
