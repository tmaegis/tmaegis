import { ArrowDown, ArrowRight, Check, FileText, Package, RotateCcw, ScanSearch, Timer, Wrench, type LucideIcon } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { PlatformIcon } from "@/components/platform-icon";

/** The systems one binary runs on (icons only; their names are listed in the tabs above). */
const SYSTEMS: [string, string][] = [
  ["windows", "Windows"],
  ["macos", "macOS"],
  ["linux", "Linux"],
  ["freebsd", "FreeBSD"],
  ["aix", "AIX"],
  ["solaris", "Solaris"],
  ["docker", "Docker"],
];

const STEPS: [key: "scan" | "report" | "fix", Icon: LucideIcon][] = [
  ["scan", ScanSearch],
  ["report", FileText],
  ["fix", Wrench],
];

function Arrow() {
  return (
    <span className="flex items-center justify-center text-primary" aria-hidden>
      <ArrowDown className="h-5 w-5 lg:hidden" />
      <ArrowRight className="hidden h-5 w-5 lg:block rtl:rotate-180" />
    </span>
  );
}

/**
 * "One binary, every platform": the download, then scan -> report -> fix, with the dead-man's switch that
 * rolls a fix back unless it is confirmed. HTML and CSS (not SVG text), so every label is translated and
 * the layout mirrors for right-to-left languages; it stacks vertically on small screens.
 */
export async function FlowDiagram() {
  const t = await getTranslations("downloads.flow");
  return (
    <figure className="glass p-5 sm:p-6">
      <ol className="grid items-stretch gap-3 lg:grid-cols-[minmax(0,1.25fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)]">
        <li className="flex min-w-0 flex-col gap-3 rounded-lg border border-primary/40 bg-primary/5 p-4">
          <div className="flex items-center gap-3">
            <span className="icon-tile"><Package className="h-5 w-5" aria-hidden /></span>
            <div className="min-w-0">
              <p className="font-semibold">{t("binary")}</p>
              <p className="font-mono text-xs text-muted-foreground" dir="ltr">tmaegis · tmaegis.exe</p>
            </div>
          </div>
          <p className="text-sm text-muted-foreground">{t("binaryText")}</p>
          <ul className="flex flex-wrap gap-1.5">
            {SYSTEMS.map(([id, name]) => (
              <li key={id} className="group" title={name}>
                <PlatformIcon id={id} size={28} tile />
                <span className="sr-only" lang="en">{name}</span>
              </li>
            ))}
          </ul>
        </li>
        {STEPS.map(([key, Icon], i) => (
          <li key={key} className="contents">
            <Arrow />
            <div className="flex min-w-0 flex-col gap-2 rounded-lg border border-border bg-card-solid/60 p-4">
              <div className="flex items-center gap-3">
                <span className="icon-tile"><Icon className="h-5 w-5" aria-hidden /></span>
                <p className="font-semibold">
                  <span className="me-1.5 font-mono text-sm text-muted-foreground">0{i + 1}</span>
                  {t(`${key}.title`)}
                </p>
              </div>
              <p className="text-sm text-muted-foreground">{t(`${key}.text`)}</p>
            </div>
          </li>
        ))}
      </ol>

      {/* the dead-man's switch: a branch after "fix" */}
      <div className="mt-3 grid gap-3 lg:ms-[calc(50%-1rem)]">
        <div className="relative rounded-lg border border-dashed border-accent/60 bg-accent/5 p-4">
          <div className="flex items-center gap-3">
            <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-accent/10 text-accent-text"><Timer className="h-5 w-5" aria-hidden /></span>
            <div className="min-w-0">
              <p className="font-semibold">{t("deadman.title")}</p>
              <p className="text-sm text-muted-foreground">{t("deadman.text")}</p>
            </div>
          </div>
          <ul className="mt-3 grid gap-2 sm:grid-cols-2">
            <li className="flex items-start gap-2 rounded-md border border-border bg-card-solid/60 p-2.5 text-sm">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-accent-text" aria-hidden />
              <span>{t("deadman.confirmed")}</span>
            </li>
            <li className="flex items-start gap-2 rounded-md border border-border bg-card-solid/60 p-2.5 text-sm">
              <RotateCcw className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden />
              <span>{t("deadman.timeout")}</span>
            </li>
          </ul>
        </div>
      </div>
      <figcaption className="mt-4 text-sm text-muted-foreground">{t("caption")}</figcaption>
    </figure>
  );
}
