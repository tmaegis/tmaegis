import { getTranslations } from "next-intl/server";

/** Illustrations drawn in CSS/SVG with sample data. Replace with captured PNGs when available. */

const row = "flex gap-2 whitespace-pre";

export async function CliMock() {
  const t = await getTranslations("home.shots.cli");
  return (
    <div className="mock" role="img" aria-label={t("alt")} dir="ltr">
      <div className="mock-bar" aria-hidden>
        <span className="mock-dot" /><span className="mock-dot" /><span className="mock-dot" />
        <span className="ms-2 font-mono">tmaegis scan</span>
      </div>
      <div className="cmd-scroll p-4 font-mono" aria-hidden>
        <div className={row}><span className="text-emerald-400">$</span><span className="text-slate-100">tmaegis scan --auto-detect</span></div>
        <div className="text-slate-500">{t("detected", { os: "Ubuntu 24.04 LTS, openssh, nginx" })}</div>
        <div className="mt-2" />
        <div lang="en" className={row}><span className="text-emerald-400">PASS</span><span>CIS-UBUNTU-5.2.10  PermitEmptyPasswords</span></div>
        <div lang="en" className={row}><span className="text-emerald-400">PASS</span><span>TMA-AUDITD-ENABLED  auditd running</span></div>
        <div lang="en" className={row}><span className="text-red-400">FAIL</span><span>CIS-LINUX-3.2.1    IP forwarding</span></div>
        <div lang="en" className={row}><span className="text-red-400">FAIL</span><span>CIS-NGINX-2.5.1    server_tokens off</span></div>
        <div className={row}><span className="text-amber-300">SKIP</span><span>CIS-UBUNTU-1.1.2    {t("manualReview")}</span></div>
        <div className="mt-2 text-slate-300">{t("summary")}  <span className="text-emerald-400">{t("passed", { count: 184 })}</span>  <span className="text-red-400">{t("failed", { count: 12 })}</span>  <span className="text-amber-300">{t("manual", { count: 9 })}</span></div>
        <div className={`${row} mt-2`}><span className="text-emerald-400">$</span><span className="text-slate-100">sudo tmaegis apply --dry-run<span className="ms-1 inline-block h-3.5 w-2 translate-y-0.5 bg-cyan-300/80" /></span></div>
      </div>
    </div>
  );
}

export async function GuiMock() {
  const t = await getTranslations("home.shots.gui");
  const r = 34;
  const c = 2 * Math.PI * r;
  const pct = 0.86;
  return (
    <div className="mock" role="img" aria-label={t("alt")} dir="ltr">
      <div className="mock-bar" aria-hidden>
        <span className="mock-dot" /><span className="mock-dot" /><span className="mock-dot" />
        <span className="ms-2">{t("windowTitle")}</span>
      </div>
      <div className="flex" aria-hidden>
        <div className="hidden w-24 shrink-0 border-e border-white/10 bg-white/[0.02] p-2 sm:block">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className={`mb-1.5 h-5 rounded ${i === 0 ? "bg-cyan-400/20" : "bg-white/5"}`} />
          ))}
        </div>
        <div className="flex-1 p-3">
          <div className="flex items-center gap-4">
            <svg width="84" height="84" viewBox="0 0 84 84" className="shrink-0">
              <circle cx="42" cy="42" r={r} fill="none" stroke="#1e293b" strokeWidth="8" />
              <circle cx="42" cy="42" r={r} fill="none" stroke="#10b981" strokeWidth="8" strokeLinecap="round" strokeDasharray={`${c * pct} ${c}`} transform="rotate(-90 42 42)" />
              <text x="42" y="47" textAnchor="middle" fontSize="16" fontWeight="600" fill="#e5e7eb">86%</text>
            </svg>
            <div className="min-w-0 space-y-1 text-[0.72rem]">
              <div className="text-slate-400">{t("score")}</div>
              <div className="flex flex-wrap gap-x-3 gap-y-1">
                <span className="text-emerald-400">{t("passing")} 184</span>
                <span className="text-red-400">{t("failing")} 12</span>
                <span className="text-amber-300">{t("manual")} 9</span>
              </div>
            </div>
          </div>
          <div className="mt-3 space-y-1.5">
            {[
              ["bg-red-400", "w-3/4"],
              ["bg-red-400", "w-2/3"],
              ["bg-amber-300", "w-4/5"],
              ["bg-amber-300", "w-1/2"],
              ["bg-emerald-400", "w-3/5"],
            ].map(([dot, w], i) => (
              <div key={i} className="flex items-center gap-2 rounded bg-white/[0.04] px-2 py-1.5">
                <span className={`h-2 w-2 shrink-0 rounded-full ${dot}`} />
                <span className={`h-2 rounded bg-white/15 ${w}`} />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export async function ReportMock() {
  const t = await getTranslations("home.shots.report");
  const bars: [string, number, string][] = [
    ["Critical", 14, "bg-red-500"],
    ["High", 38, "bg-orange-400"],
    ["Medium", 82, "bg-amber-300"],
    ["Low", 36, "bg-sky-400"],
  ];
  return (
    <div className="mock" role="img" aria-label={t("alt")} dir="ltr">
      <div className="mock-bar" aria-hidden>
        <span className="mock-dot" /><span className="mock-dot" /><span className="mock-dot" />
        <span className="ms-2">report.html</span>
      </div>
      <div className="p-4" aria-hidden>
        <div className="flex items-center gap-2">
          <span className="h-6 w-6 rounded bg-gradient-to-br from-cyan-400 to-emerald-500" />
          <span className="text-sm font-semibold text-slate-100">{t("heading")}</span>
        </div>
        <div className="mt-3 text-[0.7rem] uppercase tracking-wider text-slate-500">{t("bySeverity")}</div>
        <div className="mt-2 space-y-1.5">
          {bars.map(([label, n, color]) => (
            <div key={label} className="flex items-center gap-2 text-[0.7rem]">
              <span className="w-14 text-slate-400">{label}</span>
              <span className="h-2 flex-1 overflow-hidden rounded bg-white/10"><span className={`block h-full rounded ${color}`} style={{ width: `${n}%` }} /></span>
              <span className="w-6 text-end text-slate-300">{n}</span>
            </div>
          ))}
        </div>
        <div className="mt-3 divide-y divide-white/10 rounded border border-white/10">
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex items-center gap-2 px-2 py-1.5">
              <span className="h-2 w-10 rounded bg-white/15" />
              <span className={`h-2 flex-1 rounded bg-white/10`} />
              <span className={`h-2 w-8 rounded ${i === 0 ? "bg-red-400/60" : "bg-amber-300/60"}`} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
