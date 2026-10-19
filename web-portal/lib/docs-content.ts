import type { DocSlug } from "./site";

/**
 * Structure of the documentation pages. Everything a visitor reads lives in messages/<locale>.json under
 * `docsBody.<slug>.<key>` (en-US is the source, other locales fall back to it); this file only holds what must
 * never be translated: the order of blocks, section ids, code blocks and the first (command or flag) column of tables.
 * Inline `code` inside a message is written with backticks and must stay identical in every translation
 * (scripts/i18n-check.mjs enforces that).
 */
export type Block =
  | { t: "h2"; id: string; k: string }
  | { t: "h3"; k: string }
  | { t: "p"; k: string }
  | { t: "code"; code: string; lang?: string }
  | { t: "ul" | "ol"; keys: string[] }
  | { t: "note"; k: string }
  | { t: "table"; head: string[]; rows: { cmd: string; k: string }[] }
  | { t: "plugins" };

export const DOCS: Record<DocSlug, Block[]> = {
  install: [
    { t: "p", k: "p1" },
    { t: "h2", id: "release", k: "h_release" },
    { t: "p", k: "p2" },
    { t: "code", lang: "shell", code: `sha256sum -c checksums.txt --ignore-missing
mkdir tmaegis && tar -xzf tmaegis_<version>_linux_amd64.tar.gz -C tmaegis && cd tmaegis
./tmaegis --help` },
    { t: "h2", id: "source", k: "h_source" },
    { t: "p", k: "p3" },
    { t: "code", lang: "shell", code: `git clone https://github.com/tmaegis/tmaegis.git
cd tmaegis
make build
./bin/tmaegis --help` },
    { t: "p", k: "p4" },
    { t: "h2", id: "check", k: "h_check" },
    { t: "code", lang: "shell", code: `tmaegis discover        # operating system and installed software
tmaegis rules export -o rules.json   # the full rule catalog as JSON` },
    { t: "h2", id: "dashboard", k: "h_dashboard" },
    { t: "p", k: "p5" },
    { t: "code", lang: "shell", code: `export TMAEGIS_SERVER_TOKEN=$(openssl rand -hex 24)
docker compose up -d
tmaegis scan --auto-detect --push-url http://localhost:8080` },
    { t: "h2", id: "config", k: "h_config" },
    { t: "p", k: "p6" },
    { t: "note", k: "note1" },
  ],

  "quick-start": [
    { t: "p", k: "p1" },
    { t: "h2", id: "scan", k: "h_scan" },
    { t: "code", lang: "shell", code: `tmaegis scan --auto-detect` },
    { t: "p", k: "p2" },
    { t: "h2", id: "report", k: "h_report" },
    { t: "code", lang: "shell", code: `tmaegis scan --auto-detect --format html --output report.html
tmaegis scan --auto-detect --format sarif --output tmaegis.sarif` },
    { t: "h2", id: "preview", k: "h_preview" },
    { t: "code", lang: "shell", code: `sudo tmaegis apply --auto-detect --dry-run` },
    { t: "p", k: "p3" },
    { t: "h2", id: "apply", k: "h_apply" },
    { t: "code", lang: "shell", code: `sudo tmaegis apply --auto-detect --deadman-timeout 60` },
    { t: "p", k: "p4" },
    { t: "h2", id: "rollback", k: "h_rollback" },
    { t: "code", lang: "shell", code: `tmaegis rollback --list      # restore points
sudo tmaegis rollback        # latest snapshot
sudo tmaegis rollback --id <snapshot-id>` },
    { t: "h2", id: "cloud", k: "h_cloud" },
    { t: "code", lang: "shell", code: `tmaegis scan --target aws                             # read-only, uses your CLI credentials
tmaegis apply --target aws --terraform-out ./proposed  # writes Terraform, never changes the account` },
    { t: "h2", id: "network", k: "h_network" },
    { t: "code", lang: "shell", code: `tmaegis scan --target cisco-ios --device-config r1.cfg` },
    { t: "note", k: "note1" },
  ],

  cli: [
    { t: "p", k: "p1" },
    { t: "h2", id: "global", k: "h_global" },
    { t: "table", head: ["t1_h1","t1_h2"], rows: [
      { cmd: "`-c, --config`", k: "t1_r1" },
      { cmd: "`-r, --rules-dir`", k: "t1_r2" },
      { cmd: "`--lang`", k: "t1_r3" },
      { cmd: "`--ai-provider`", k: "t1_r4" },
      { cmd: "`--os-family`", k: "t1_r5" },
    ] },
    { t: "h2", id: "scan", k: "h_scan" },
    { t: "p", k: "p2" },
    { t: "table", head: ["t2_h1","t2_h2"], rows: [
      { cmd: "`--auto-detect`", k: "t2_r1" },
      { cmd: "`-t, --target`", k: "t2_r2" },
      { cmd: "`-l, --level`", k: "t2_r3" },
      { cmd: "`-f, --format`", k: "t2_r4" },
      { cmd: "`-o, --output`", k: "t2_r5" },
      { cmd: "`--include-manual`", k: "t2_r6" },
      { cmd: "`--device-config`", k: "t2_r7" },
      { cmd: "`--ai-analyze`", k: "t2_r8" },
      { cmd: "`--plugin`", k: "t2_r9" },
      { cmd: "`--push-url`", k: "t2_r10" },
      { cmd: "`-q, --quiet`", k: "t2_r11" },
    ] },
    { t: "h2", id: "apply", k: "h_apply" },
    { t: "p", k: "p3" },
    { t: "table", head: ["t3_h1","t3_h2"], rows: [
      { cmd: "`--auto-detect`, `-t`, `-l`", k: "t3_r1" },
      { cmd: "`--dry-run`", k: "t3_r2" },
      { cmd: "`--deadman-timeout`", k: "t3_r3" },
      { cmd: "`--no-confirm`", k: "t3_r4" },
      { cmd: "`--auto-rollback`", k: "t3_r5" },
      { cmd: "`--terraform-out`", k: "t3_r6" },
    ] },
    { t: "h2", id: "rollback", k: "h_rollback" },
    { t: "table", head: ["t4_h1","t4_h2"], rows: [
      { cmd: "`--list`", k: "t4_r1" },
      { cmd: "`--id`", k: "t4_r2" },
    ] },
    { t: "h2", id: "other", k: "h_other" },
    { t: "table", head: ["t5_h1","t5_h2"], rows: [
      { cmd: "`tmaegis discover [--json]`", k: "t5_r1" },
      { cmd: "`tmaegis rules export [-o file]`", k: "t5_r2" },
      { cmd: "`tmaegis plugin install|list`", k: "t5_r3" },
      { cmd: "`tmaegis lang`", k: "t5_r4" },
    ] },
    { t: "h2", id: "exit", k: "h_exit" },
    { t: "p", k: "p4" },
  ],

  "rules-format": [
    { t: "p", k: "p1" },
    { t: "h2", id: "example", k: "h_example" },
    { t: "code", lang: "yaml", code: `id: "CIS-UBUNTU-5.2.8"
title: "Ensure SSH root login is disabled"
description: "The PermitRootLogin parameter specifies if the root user can log in using ssh."
target: "linux"
os_family: ["ubuntu-22.04", "ubuntu-24.04", "rocky-9"]
level: 1
severity: "high"
tags: ["ssh", "server"]
audit:
  type: "file_regex"
  file_path: "/etc/ssh/sshd_config"
  pattern: '(?m)^\\s*PermitRootLogin\\s+no\\b'
remediation:
  type: "command"
  command: "sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config"
  verify_cmd: "sshd -t"
  backup_files: ["/etc/ssh/sshd_config"]` },
    { t: "h2", id: "fields", k: "h_fields" },
    { t: "table", head: ["t1_h1","t1_h2"], rows: [
      { cmd: "`id`", k: "t1_r1" },
      { cmd: "`title`, `description`", k: "t1_r2" },
      { cmd: "`target`", k: "t1_r3" },
      { cmd: "`os_family`", k: "t1_r4" },
      { cmd: "`level`", k: "t1_r5" },
      { cmd: "`severity`", k: "t1_r6" },
      { cmd: "`tags`, `reference`", k: "t1_r7" },
      { cmd: "`audit`", k: "t1_r8" },
      { cmd: "`remediation`", k: "t1_r9" },
    ] },
    { t: "h2", id: "audit", k: "h_audit" },
    { t: "table", head: ["t2_h1","t2_h2"], rows: [
      { cmd: "`command`", k: "t2_r1" },
      { cmd: "`file_regex`", k: "t2_r2" },
      { cmd: "`config_regex`", k: "t2_r3" },
      { cmd: "`sql_query`", k: "t2_r4" },
      { cmd: "`registry`", k: "t2_r5" },
      { cmd: "`cloud_check`", k: "t2_r6" },
      { cmd: "`manual`", k: "t2_r7" },
    ] },
    { t: "h2", id: "remediation", k: "h_remediation" },
    { t: "table", head: ["t3_h1","t3_h2"], rows: [
      { cmd: "`command`", k: "t3_r1" },
      { cmd: "`file_line`", k: "t3_r2" },
      { cmd: "`sql_exec`", k: "t3_r3" },
      { cmd: "`iac_patch`", k: "t3_r4" },
    ] },
    { t: "note", k: "note1" },
    { t: "h2", id: "browse", k: "h_browse" },
    { t: "p", k: "p2" },
  ],

  plugins: [
    { t: "p", k: "p1" },
    { t: "h2", id: "use", k: "h_use" },
    { t: "code", lang: "shell", code: `tmaegis plugin install github.com/tmaegis/plugin-slack-notify   # file, URL or GitHub release
tmaegis plugin install ./my-plugin.wasm --sha256 <digest>       # pin the digest
tmaegis plugin list
tmaegis scan --auto-detect --plugin slack,jira` },
    { t: "p", k: "p2" },
    { t: "h2", id: "write", k: "h_write" },
    { t: "p", k: "p3" },
    { t: "code", lang: "shell", code: `#!/usr/bin/env sh
# TMAegis notifier plugin: posts a scan summary somewhere.
set -eu
PAYLOAD=$(cat)
FAILED=$(printf '%s' "$PAYLOAD" | grep -o '"failed_count":[0-9]*' | head -n1 | cut -d: -f2)
echo "failed controls: \${FAILED:-0}"` },
    { t: "p", k: "p4" },
    { t: "h2", id: "list", k: "h_list" },
    { t: "plugins" },
  ],

  reports: [
    { t: "p", k: "p1" },
    { t: "table", head: ["t1_h1","t1_h2"], rows: [
      { cmd: "`text`", k: "t1_r1" },
      { cmd: "`html`", k: "t1_r2" },
      { cmd: "`pdf`", k: "t1_r3" },
      { cmd: "`xlsx`", k: "t1_r4" },
      { cmd: "`sarif`", k: "t1_r5" },
      { cmd: "`asff`", k: "t1_r6" },
      { cmd: "`json`, `jsonl`", k: "t1_r7" },
      { cmd: "`csv`", k: "t1_r8" },
    ] },
    { t: "h2", id: "example", k: "h_example" },
    { t: "code", lang: "shell", code: `tmaegis scan --auto-detect --format html --output report.html
tmaegis scan --target linux --format sarif --output tmaegis.sarif
tmaegis scan --format json > report.json` },
    { t: "h2", id: "language", k: "h_language" },
    { t: "p", k: "p2" },
    { t: "h2", id: "results", k: "h_results" },
    { t: "ul", keys: ["l1_1","l1_2","l1_3"] },
  ],

  gui: [
    { t: "p", k: "p1" },
    { t: "h2", id: "start", k: "h_start" },
    { t: "code", lang: "shell", code: `tmaegis gui` },
    { t: "p", k: "p2" },
    { t: "h2", id: "flow", k: "h_flow" },
    { t: "ol", keys: ["l1_1","l1_2","l1_3","l1_4","l1_5"] },
    { t: "note", k: "note1" },
  ],
};
