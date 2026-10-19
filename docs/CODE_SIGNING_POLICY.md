# Code signing policy

Free code signing for the Windows builds of TMAegis is provided by [SignPath.io](https://signpath.io),
certificate by [SignPath Foundation](https://signpath.org).

## What is signed

Only binaries built from this repository's source code by its GitHub Actions workflows
(`.github/workflows/release.yml` and `.github/workflows/desktop.yml`): `tmaegis.exe` (command line) and
`tmaegis-desktop.exe` (desktop app), inside the Windows `.zip` release archives. Third-party code is built from
source and is never signed on behalf of its authors. Product name metadata is `TMAegis` and the product version is the release version.

## Roles

| Role | Who | Responsibility |
|---|---|---|
| Authors (committers) | Tommy Huynh ([@tnptommy](https://github.com/tnptommy)), Misha Milanovic | Write and merge code. |
| Reviewers | Tommy Huynh, Misha Milanovic | Review every change from anyone who is not a committer before it is merged. |
| Approvers | Tommy Huynh, Misha Milanovic | Approve each signing request after checking that the release was built from the tagged commit. |

Everyone with write access to the repository or access to the signing service uses multi-factor authentication.

## Release rules

- Signing requests come only from the release workflow running on GitHub-hosted runners, never from a developer machine.
- Every release signing request is approved manually by an approver.
- Release archives are published with SHA-256 checksums (`checksums.txt`) that are regenerated after signing.

## Privacy

TMAegis does not send information to other networked systems unless the user asks for it (for example `--push-url`, an AI
provider the user configures, or an SSH/cloud target the user names). See the [privacy summary](https://www.tmaegis.dev/legal#privacy).

## Reporting a problem

If you think a signed file is malicious or was signed by mistake, write to security@tmaegis.dev or use GitHub private
vulnerability reporting (see [SECURITY.md](../SECURITY.md)).
