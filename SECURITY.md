# Security policy

## Reporting a vulnerability

Please **do not** open a public issue for a security problem in TMAegis.

- Use GitHub's **Report a vulnerability** button (Security tab of
  [tmaegis/tmaegis](https://github.com/tmaegis/tmaegis/security/advisories/new)), or
- email **security@tmaegis.dev** with the steps to reproduce, the affected version (`tmaegis --version`)
  and the impact.

We acknowledge reports within 3 working days, agree a fix and disclosure date with you, and credit
you in the release notes unless you prefer to stay anonymous.

## Scope

In scope: the `tmaegis` binary (CLI, local GUI, reports), `tmaegis-server`, the rule engine, the
official rules in `rules/`, release artifacts and www.tmaegis.dev.

A rule that reports a wrong PASS/FAIL is a bug, not a vulnerability, unless it can be abused (for
example an audit command that changes the system or runs untrusted input).

## Verifying downloads

Every release publishes `checksums.txt` (SHA-256). Verify before running:

```powershell
Get-FileHash .\tmaegis_0.1.0_windows_amd64.zip -Algorithm SHA256
```
```bash
sha256sum -c checksums.txt --ignore-missing
```

Only download TMAegis from www.tmaegis.dev or https://github.com/tmaegis/tmaegis/releases.
