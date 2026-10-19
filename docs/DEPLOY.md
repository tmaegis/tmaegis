# Deploying & Running the Website (`web-portal/`)

The public TMAegis website lives in [`web-portal/`](../web-portal/) (Next.js 16, `next-intl` with 16 locales, Tailwind CSS 4, Geist typography). It is a **hybrid Next.js application**:
- **Static pages** for localized landing, features, coverage, documentation, pricing, contact, and legal pages across all 16 locales (`/en`, `/en-gb`, `/vi`, `/zh-hans`, `/zh-hant`, `/es`, `/fr`, `/de`, `/ja`, `/ko`, `/ru`, `/tr`, `/pt-br`, `/id`, `/sr`, `/sr-cyrl`).
- **ISR (Incremental Static Regeneration)** for individual rule pages and the `/downloads` page (10-minute revalidation against the GitHub Releases API).
- **Server & Edge routes** for `/api/rules/search`, `/api/contact`, `/api/health`, and `/og` (dynamic Open Graph social cards).

It is **not** a static `output: "export"` build.

---

## 1. Running Locally

Prerequisites: **Go 1.27+** and **Node.js 24+**.

```bash
cd web-portal
npm ci
npm run dev
```

`npm run dev` automatically runs `scripts/gen-data.mjs` (which invokes `go run . rules export` from the repository root to build the compact search index and per-target rule shards) and starts the local development server at `http://localhost:3000`.

Useful scripts inside `web-portal/`:

| Command | Description |
|---|---|
| `npm run gen` | Exports all rules from the Go CLI and writes `public/data/` + `generated/`. |
| `npm run dev` | Generates site data and starts Next.js (Turbopack) on `http://localhost:3000`. |
| `npm run i18n:check` | Validates all 16 `messages/<tag>.json` files against `en-US.json` (keys, ICU placeholders, CLDR plural rules). |
| `npm run lint` | Runs ESLint across `web-portal/`. |
| `npm run typecheck` | Runs TypeScript (`tsc --noEmit`). |
| `npm test` | Runs `i18n:check` followed by Vitest unit tests (`vitest run`). |
| `npm run build` | Generates site data (unless `SKIP_GEN=1` is set) and runs a production Next.js build. |

---

## 2. Cloudflare builds the site from the public repository

The site is hosted on Cloudflare and built by Cloudflare from the **public** repository `tmaegis/tmaegis`
(every push to `main` is a production deployment). There is no deploy workflow in GitHub Actions and no
deploy secret in GitHub.

1. **Rule data is committed.** The rule catalog and coverage numbers come from the Go CLI
   (`go run . rules export`), which Cloudflare's build image may not have. After changing rules run
   `python3 scripts/gen_coverage.py` and `cd web-portal && npm run gen` and commit `web-portal/generated/`,
   `web-portal/public/data/` and `docs/COVERAGE.md`. Cloudflare then builds with `SKIP_GEN=1`.
2. **CIS PDFs never reach the site.** `benchmarks/` and `rules-staging/` are excluded from the public repository
   by `scripts/publish_public.sh`, so the Cloudflare build cannot see them.
3. **CI still checks the site.** `.github/workflows/ci.yml` (job `web-portal`, public repository only) runs
   `npm ci`, `npm run gen`, lint, typecheck, tests and a production build.

---

## 3. One-Time Cloudflare Setup

1. In Cloudflare (**Workers & Pages -> Create -> Connect to Git**) select `tmaegis/tmaegis`, set the root
   directory to **`web-portal`**, the build command to `SKIP_GEN=1 npm run build` (or the OpenNext build command
   if you deploy Next.js on Workers) and Node.js 24.
2. Add the environment variables from section 4 under the project's **Settings -> Variables and Secrets**.
3. Push to `main`; Cloudflare builds and deploys. Preview deployments are created for other branches.

---

## 4. Environment Variables

Configure these in **Cloudflare -> Workers & Pages -> your project -> Settings -> Variables and Secrets**:

| Variable | Required | Purpose |
|---|---|---|
| `SITE_URL` | Recommended | Canonical origin (`https://www.tmaegis.dev`; the code already defaults to it in production). Used for canonical URLs, `hreflang` alternates, `sitemap.xml`, `robots.txt`, and Open Graph cards. Preview deployments fall back to their preview URL. |
| `CONTACT_WEBHOOK_URL` | For contact form | Slack-compatible incoming webhook URL (`{"text": "..."}`). If unset, `/api/contact` returns HTTP `503` and the contact page directs visitors to email. |
| `NEXT_PUBLIC_CONTACT_EMAIL` | Optional | Contact email displayed on `/contact` (defaults to `contact@tmaegis.dev`). |
| `NEXT_PUBLIC_GITHUB_REPO` | Optional | `owner/repo` used for GitHub links and `/downloads` release lookups (defaults to `tmaegis/tmaegis`). |
| `GITHUB_TOKEN` | Optional | Raises the GitHub API rate limit when fetching releases on `/downloads`. Not needed for the public repository; only set it for a private fork. |

---

## 5. Custom Domain Setup

1. In Cloudflare (**your project -> Custom domains**) add `www.tmaegis.dev`; Cloudflare creates the DNS record
   because the domain's DNS is already on Cloudflare.
2. Make the apex `tmaegis.dev` redirect to `www.tmaegis.dev` (a Redirect Rule or Bulk Redirect). `SITE_URL=https://www.tmaegis.dev`
   is already the default in production.

---

## 6. Generated Data & Security Headers

- **Generated Artifacts (`npm run gen`, git-ignored):**
  - `public/data/index.json` — Compact search index (~603 KiB for 8,597 rules).
  - `public/data/targets/<target>.json` — Per-platform rule shards so no single giant JSON bundle is loaded.
  - `generated/` — Coverage totals, plugin registry metadata, and copies of `TRADEMARK.md` and `NOTICE`.
- **Security & Privacy:**
  - [`web-portal/vercel.json`](../web-portal/vercel.json) and [`web-portal/next.config.mjs`](../web-portal/next.config.mjs) enforce strict `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, `Referrer-Policy`, and `Permissions-Policy` headers.
  - The website uses **zero analytics or third-party tracking scripts**, self-hosts all fonts via `next/font`, and sets only a single functional cookie (`NEXT_LOCALE`) when a user switches languages.

---

## 7. Where the downloads come from

**The website does not host any binaries.** `web-portal/public/releases/` no longer exists (it is git-ignored and
excluded from deployments); every Download button links straight to a file on GitHub Releases.

1. Push the tag in the private repository (see [RELEASING.md](RELEASING.md)): `git tag v0.1.0 && git push origin v0.1.0`.
2. [`publish-public.yml`](../.github/workflows/publish-public.yml) verifies the code and pushes a clean snapshot plus the
   tag `v0.1.0` to the public repository `tmaegis/tmaegis`.
3. In the public repository the tag triggers [`release.yml`](../.github/workflows/release.yml), which runs GoReleaser
   ([`.goreleaser.yaml`](../.goreleaser.yaml)). It publishes a GitHub Release with
   `tmaegis_<version>_<os>_<arch>.zip` (Windows) / `.tar.gz` (Linux, macOS) for `amd64` and `arm64`, the Linux packages
   `tmaegis_<version>_linux_<arch>.deb` / `.rpm` (nfpm; they install `/usr/bin/tmaegis` and `/usr/share/tmaegis/rules`), and
   `checksums.txt` (SHA-256). Each archive holds `tmaegis`, `tmaegis-server`, the rules, the plugins, `samples/`, the license
   files and `Start-TMAegis-GUI.cmd`. It also pushes the multi-arch image `ghcr.io/tmaegis/tmaegis:<version>` and `:latest`.
4. `/downloads` reads the latest release from `https://api.github.com/repos/tmaegis/tmaegis/releases` (revalidated every 10
   minutes), reads `checksums.txt` for the SHA-256 shown on each card, and lists up to 10 earlier versions.
   No token is needed for a public repository. It also asks GHCR anonymously whether `ghcr.io/tmaegis/tmaegis:<version>`
   exists (`checkImage` in `lib/releases.ts`) and shows the Docker tab as published, not in this release, or not checked.
   Every install option (zip, tar.gz, .deb, .rpm, image) is shown only when the release lists it; otherwise the card says
   "not in this release" and has no link and no command. Homebrew, winget, Scoop and .msi/.pkg are listed as planned only.
5. The "Test your device" section is driven by [`web-portal/lib/device-guides.ts`](../web-portal/lib/device-guides.ts)
   (one guide per scan target: mode, where to run it, vendor export or sign-in commands). `lib/device-guides.test.ts`
   fails when `generated/coverage.json` has a target without a guide.

Graceful states of the page:

| Situation | What visitors see |
|---|---|
| GitHub answers with a release | Live data: sizes, SHA-256, release notes, earlier versions |
| GitHub is unreachable, rate-limited or answers with an error | The bundled description of v0.1.0 in [`web-portal/lib/release-fallback.json`](../web-portal/lib/release-fallback.json) (URLs on `github.com/tmaegis/tmaegis/releases/download/v0.1.0/`, no sizes or hashes), with a notice |
| GitHub answers but there is no published release yet (or the repository is not public yet) | "The first public release is coming soon" with build-from-source steps |

When the version changes, update `release-fallback.json` (tag, name and file names, including the planned `.deb`/`.rpm`) together with the version bump in
[RELEASING.md](RELEASING.md); the unit test `lib/releases.test.ts` checks that the fallback still lists all six builds.
Nothing else on the site needs to change for a release.
