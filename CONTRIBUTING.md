# Contributing to TMAegis™

Thank you for helping improve TMAegis. This guide covers development workflows, rule authoring, localization (16 languages), and the legal terms that apply to every contribution.

---

## Ways to Contribute

- **Report bugs and false positives/negatives:** A rule that passes or fails inaccurately on a real host is a bug.
- **Author or improve rules:** Fix a threshold, tighten a regex, add a missing control, or review staged rules from `rules-staging/`.
- **Translations & Localization (16 Locales):** Review or improve translations across the CLI, Desktop GUI, HTML/PDF reports (`internal/i18n/locales/`), and the Website (`web-portal/messages/`).
- **Plugins & Integrations:** Add or enhance executable or WebAssembly (`.wasm`) plugins in `plugins/`.
- **Engine, GUI & Website:** Improve the Go engine, local GUI, report exporters, or Next.js web portal.

Open an issue first for large architectural changes so we can align on the design before you spend time coding.

---

## Development & Verification Commands

Run these checks locally before opening a pull request:

```bash
# 1. Go CLI, engine, GUI, and report unit tests
go build ./... && go test ./...
go vet ./...

# 2. Validate all 8,597+ rules in rules/ (schema, unique IDs, read-only audit safety)
go run . rules validate

# 3. Regenerate docs/COVERAGE.md if rules were added, removed, or recategorized
python3 scripts/gen_coverage.py

# 4. Python CIS PDF converter & compiler tests
cd ai-pipeline && python3 -m pytest

# 5. Website (Next.js 16 + 16-locale i18n check + Vitest + TypeScript + ESLint)
cd web-portal
npm ci
npm run gen
npm run lint
npm run typecheck
npm test
SKIP_GEN=1 npm run build
```

---

## Writing or Updating Rules (`rules/`)

Every rule is a standalone YAML file under `rules/<category>/<target>/`. A rule must have:

1. **Unique ID:** `TMA-<TARGET>-<section>` (uppercase target prefix, matching the benchmark or topic section).
2. **Accurate Metadata:** `title`, `description`, `level` (`1` or `2`), `severity` (`critical`, `high`, `medium`, or `low`), `target`, and `os_family` (use `["*"]` when OS-agnostic or a specific family like `["ubuntu-22.04"]`).
3. **Read-Only Audit:** Audits (`command`, `file_regex`, `config_regex`, `registry`, `sql_query`, `cloud_check`, or `manual`) must **never** modify system state, restart services, or write files.
4. **Safe Remediation:** Use `iac_patch` (human-readable remediation steps or Terraform guidance) unless an automated fix (`command`, `file_line`, `sql_exec`) is deterministic, idempotent, and safe to roll back.
5. **Benchmark Reference:** Include a `reference` citing the benchmark name, version, and control number.

Test new or modified rules on a real target host or with unit tests, and document how you tested them in your pull request description.

---

## Localization & Translations (16 Locales)

TMAegis supports 16 locales across every surface (`en-US`, `en-GB`, `vi`, `zh-Hans`, `zh-Hant`, `es-419`, `fr`, `de`, `ja`, `ko`, `ru`, `tr`, `pt-BR`, `id`, `sr-Latn`, `sr-Cyrl`):

- **CLI, Desktop GUI & Reports:** Flat JSON catalogs in `internal/i18n/locales/<tag>/` (`cli.json`, `gui.json`, `report.json`). Verified by `go test ./internal/i18n/...`.
- **Website (`web-portal`):** Nested ICU MessageFormat catalogs in `web-portal/messages/<tag>.json`. Verified by `cd web-portal && npm run i18n:check`, which validates JSON syntax, ICU placeholders, and CLDR plural categories against `en-US.json`.
- **Native Speaker Review (`Reviewed` status):** Locales awaiting full native-speaker sign-off are marked with `· Beta` in language pickers (configured in [`internal/i18n/status.go`](internal/i18n/status.go)). Once a native speaker reviews all catalogs (`cli`, `gui`, `report`, and `web-portal`) for a locale, its tag can be added to `i18n.Reviewed`.

---

## Legal Terms for Contributions (DCO and CLA)

TMAegis is licensed under the [Mozilla Public License 2.0](LICENSE). By submitting a contribution (a pull request, patch, issue attachment, or any other material) to the project, **you agree to all of the following**:

1. **Developer Certificate of Origin 1.1:** You certify the statements at <https://developercertificate.org/>: you wrote the contribution or otherwise have the right to submit it under the licence below, and you understand that your contribution and sign-off are public and preserved indefinitely.
2. **Licence:** Your contribution is licensed under **MPL-2.0**, the same licence as the project.
3. **Rights Granted to the Maintainers (CLA):** You grant the project maintainers, the Founders (Tommy Huynh and Misha Milanovic) and their successors, joint copyright rights in your contribution: a perpetual, worldwide, royalty-free, irrevocable licence to use, copy, modify, distribute, and **sublicense** it, including under other licences (for example a commercial licence), to the extent you hold the rights to do so, and joint ownership of the copyright in it where the law of your country allows an assignment. You retain the right to use your own contribution.
4. **No Trademark Rights:** Contributing grants you **no** rights in the TMAegis™ name or the TM Cyber-Shield logo, and you do not claim any. They remain the property of the Founders. See [`TRADEMARK.md`](TRADEMARK.md).
5. **Third-Party Material:** Do not submit code, text, or data you do not have the right to submit. In particular, do **not** commit CIS Benchmark PDFs or restricted material into the repository outside the places agreed with the maintainers (see [`NOTICE`](NOTICE)).
6. **Employer Permission:** If your employer holds rights to what you write, you confirm that you have permission to contribute under these terms.

### How to Sign Off Your Commits

Sign off every commit with `git commit -s`, which appends:

```text
Signed-off-by: Your Real Name <you@example.com>
```

Pull requests containing unsigned commits cannot be merged.

---

## Code Style & Security

- **Go:** Format with `gofmt`, check with `go vet`, and begin every new `.go` file with the 3-line copyright, `SPDX-License-Identifier: MPL-2.0`, and trademark header.
- **No Secrets:** Never include credentials, API keys, internal hostnames, or customer data in code, rules, tests, or commit messages.
- **Reporting Vulnerabilities:** Please do **not** open a public issue for security vulnerabilities in TMAegis. Report vulnerabilities privately to `security@tmaegis.dev` (see [`AUTHORS`](AUTHORS)) or via GitHub's private vulnerability reporting.
