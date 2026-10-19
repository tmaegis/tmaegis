# Releasing TMAegis

Development happens in the **private** repository (`tnptommy/tmaegis`). The **public** repository
(`tmaegis/tmaegis`) receives a clean snapshot (no `ai-pipeline/`, `rules-staging/`, `benchmarks/`,
`scripts/`, no history) every time a release tag is pushed.

```
private repo: git tag v0.1.0 && git push origin v0.1.0
   └─ .github/workflows/publish-public.yml   (build + tests + rule validation, then publish)
        └─ public repo: main + tag v0.1.0
             └─ .github/workflows/release.yml (GoReleaser) -> GitHub Release with archives + checksums.txt
                  └─ www.tmaegis.dev/downloads shows the new release (refreshes every 10 minutes)
```

## One-time setup: give the workflow permission to write to the public repo

The `tmaegis` organization **disables deploy keys** by policy, so use a **GitHub App** (recommended by
GitHub: tokens last one hour, and there is no long-lived key to leak).

### A. GitHub App (recommended)

1. Organization `tmaegis` -> **Settings** -> **Developer settings** -> **GitHub Apps** -> **New GitHub App**.
   - **GitHub App name:** `tmaegis-publisher` (any free name)
   - **Homepage URL:** `https://www.tmaegis.dev`
   - **Webhook:** untick **Active**
   - **Repository permissions:** **Contents: Read and write**, **Workflows: Read and write**
     (Workflows is required because the snapshot contains files under `.github/workflows/`), Metadata: Read-only (automatic)
   - **Where can this GitHub App be installed?** *Only on this account*
   - **Create GitHub App**.
2. On the app page note the **App ID** (a number) and click **Generate a private key**: a `.pem` file downloads.
3. Left menu **Install App** -> install on organization `tmaegis` -> **Only select repositories** -> `tmaegis/tmaegis`.
4. Private repo `tnptommy/tmaegis` -> **Settings** -> **Secrets and variables** -> **Actions** -> two repository secrets:
   - `PUBLISHER_APP_ID` = the App ID
   - `PUBLISHER_APP_PRIVATE_KEY` = the full content of the `.pem` file (including the BEGIN/END lines)
   Then delete the `.pem` from your PC.
5. Recommended: private repo -> **Settings** -> **Environments** -> `public-release` -> **Required reviewers**
   (Tommy and/or Misha): every publish then waits for one click of approval.
6. If the public `main` branch is protected by a rule or ruleset, add the app to its **bypass list**
   (the snapshot replaces `main` with a force push).
7. Add `tommy.huynh@tmaegis.dev` and `misha.milanovic@tmaegis.dev` to your GitHub accounts
   (Settings -> Emails) so the public commits link to your profiles.

### B. Deploy key (only if the organization allows deploy keys)

Ask an owner to allow deploy keys in the organization's security settings, then:
`ssh-keygen -t ed25519 -C "tmaegis-public-publisher" -f tmaegis_public_deploy -N ""`; add the `.pub` file to
`tmaegis/tmaegis` -> Settings -> Deploy keys with **Allow write access**; store the private key in the
secret `PUBLIC_REPO_DEPLOY_KEY`; delete both files from your PC. The workflow uses the GitHub App when its
secrets exist and falls back to the deploy key otherwise.

## Every release

1. Bump the version in **three** places (the workflow refuses to publish if they differ from the tag):
   `cmd/root.go` (`Version`), `Makefile` (`VERSION=`), `web-portal/package.json` (`"version"`).
   For Windows file properties also run `python scripts/gen_winres.py` after editing its version.
2. Commit and push to the private repository, then publish from your PC (see *Free setup* below). The public
   repository builds the release, the desktop apps, the Docker image and runs CI; none of that runs in the private one.
3. Watch *Actions -> Release* in the public repository.

The *Publish public snapshot* workflow is manual only (it needs Actions minutes in the private repository). You can run it by hand (Actions -> Run workflow) with **dry run** to check
a snapshot without pushing, or with **remove old tags** once to delete `v0.4.0` / `v2.4.0`.
Old GitHub Release pages (e.g. v2.4.0) must be deleted in the public repo's web UI.

## Free setup: run everything in the public repository

The private repository has no free Actions minutes, so only the public repository runs CI, the
website deploy, releases and the desktop build (public repositories use Actions for free).
Publish from your own PC instead of the *Publish public snapshot* workflow:

1. One-time: create a deploy key (see B above), add the `.pub` file to `tmaegis/tmaegis` -> Settings ->
   Deploy keys with **Allow write access**, and keep the private key only on your PC.
2. The website needs no secret: Cloudflare builds it from the public repository after each publish.
3. Each release (after bumping the version as above):
   ```bash
   DRY_RUN=true  PUBLIC_REPO_URL=git@github.com:tmaegis/tmaegis.git ./scripts/publish_public.sh   # check first
   GIT_SSH_COMMAND="ssh -i ~/.ssh/tmaegis_public_deploy" PUBLIC_REPO_URL=git@github.com:tmaegis/tmaegis.git VERSION=0.1.0 ./scripts/publish_public.sh
   ```
4. Run the internal Python checks before publishing: `make ai-pipeline-test`.

## What the publish step refuses

- any of the internal directories in the snapshot, any `.pdf`, any file larger than 50 MB;
- strings that look like secrets (AWS keys, GitHub tokens, private keys, API keys);
- a tag that does not match the version in `cmd/root.go`, `Makefile` and `web-portal/package.json`.

`scripts/publish_public.ps1` does the same from a Windows PC if you ever need to publish manually.

## Windows code signing (SignPath Foundation)

1. Apply at https://signpath.org (Foundation, free for open source). The project must publish a code signing policy
   (`docs/CODE_SIGNING_POLICY.md`, shown on the website at /legal#code-signing and linked from the Downloads page).
2. After acceptance, in SignPath create the project (slug `tmaegis`), upload `.signpath/artifact-configurations/windows-zip.xml`
   as the artifact configuration (slug `windows-zip`) and a signing policy (slug `release-signing`, manual approval required).
3. In the public repo (tmaegis/tmaegis): secret `SIGNPATH_API_TOKEN`, variables `SIGNPATH_ORGANIZATION_ID` (and optionally
   `SIGNPATH_PROJECT_SLUG`, `SIGNPATH_SIGNING_POLICY_SLUG`). Install the SignPath GitHub app on the repository.
4. After the Release and Desktop workflows finished for a tag, run *Actions -> Sign Windows builds* with the tag; approve the
   request in SignPath. The workflow replaces the Windows zips and regenerates `checksums.txt`.
The workflow was written without access to a SignPath account: check the first run end to end.
