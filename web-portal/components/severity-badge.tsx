import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";

const ICON: Record<string, string> = { critical: "▲▲", high: "▲", medium: "◆", low: "●" };

/** Severity shown as text plus a shape, never colour alone. */
export function SeverityBadge({ severity }: { severity: string }) {
  const t = useTranslations("rules.sev");
  const variant = (["critical", "high", "medium", "low"].includes(severity) ? severity : "default") as "critical" | "high" | "medium" | "low" | "default";
  return (
    <Badge variant={variant}>
      <span aria-hidden className="text-[0.6rem] leading-none">{ICON[severity] ?? "●"}</span>
      {t(severity)}
    </Badge>
  );
}
