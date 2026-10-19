## What does this change?

<!-- A short description of what changed and why. Link the issue if there is one (e.g. Fixes #123). -->

## How was it tested?

<!-- Commands run, target OS/host tested on, or unit tests added. For new or modified rules: what passed and what failed. -->

## Checklist

- [ ] `go build ./... && go test ./...` and `go run . rules validate` pass
- [ ] If `web-portal/` or `messages/` changed: `cd web-portal && npm run lint && npm run typecheck && npm test` pass
- [ ] If rules in `rules/` were added or removed: `python3 scripts/gen_coverage.py` was run to update `docs/COVERAGE.md`
- [ ] New or changed rules are strictly read-only to audit, have a unique `TMA-*` ID, and cite a benchmark reference
- [ ] New Go files start with the 3-line copyright, `SPDX-License-Identifier: MPL-2.0`, and trademark header
- [ ] I did not commit CIS Benchmark PDFs, secrets, or third-party material that I do not have the right to submit

## Contribution terms (DCO / CLA)

- [ ] All my commits are signed off (`git commit -s`), and **I agree to the terms in [CONTRIBUTING.md](../CONTRIBUTING.md)**. In particular, I certify the Developer Certificate of Origin 1.1; I license my contribution under **MPL-2.0**; I grant the project maintainers joint copyright rights in it as described there; and I claim no trademark rights in TMAegis™ or the TM Cyber-Shield logo.
