# Remote auditing over SSH (phase 1: read-only)

`tmaegis scan` can audit another machine over SSH. It runs the same rules from the rules
directory against a remote Linux/Unix host, or fetches the configuration of a network device
and evaluates it exactly like `--device-config` does for a saved file.

**Phase 1 is strictly read-only.** Nothing is remediated remotely: `tmaegis apply --ssh` is
refused with an error, and the engine refuses remediation through a remote executor even if
it were called. Audit remotely, then remediate on the host itself (or through your
configuration management).

> Only scan systems you are authorised to assess. tmaegis prints this reminder to stderr at
> the start of every remote scan (`--quiet` suppresses it).

## Usage

```bash
# One Linux host: os_family comes from the remote /etc/os-release
tmaegis scan --ssh auditor@web01.example.com
tmaegis scan --ssh auditor@10.0.0.5:2222 --target linux --level 2 -f html -o web01.html

# A network device (the vendor command table below decides what is sent)
tmaegis scan --ssh netops@core1 --target cisco-ios
tmaegis scan --ssh netops@mx1 --target juniper-junos -f json -o mx1.json

# A vendor whose command is not verified: say exactly what to run
tmaegis scan --ssh admin@pa1 --target paloalto \
  --ssh-command 'set cli pager off' --ssh-command 'set cli config-output-format set' \
  --ssh-command 'show config running'

# Through a bastion, with an explicit key and known_hosts file
tmaegis scan --ssh auditor@db1 --ssh-jump jump@bastion.example.com \
  --ssh-key ~/.ssh/audit_ed25519 --ssh-known-hosts ./known_hosts

# Many hosts: one report per host plus a combined summary on stderr
tmaegis scan --ssh-inventory hosts.txt -f json -o ./reports --ssh-parallel 4
```

| Flag | Meaning |
| --- | --- |
| `--ssh [user@]host[:port]` | Destination. The user defaults to your local user name; IPv6 needs brackets with a port (`[2001:db8::1]:22`). |
| `--ssh-key <file>` | Private key. Without it: ssh-agent (`SSH_AUTH_SOCK`), then `~/.ssh/id_ed25519`, `id_ecdsa`, `id_rsa`. |
| `--ssh-known-hosts <file>` | Host keys to trust (default `~/.ssh/known_hosts`). |
| `--ssh-timeout <duration>` | Connect and handshake timeout (default `15s`). |
| `--ssh-jump [user@]host[:port]` | Bastion host; its key is verified and it authenticates like the target. |
| `--ssh-command '<cmd>'` | Device command(s) to fetch the configuration (repeatable). Required for unverified vendors. |
| `--ssh-inventory <file>` | Scan every host in the file. |
| `--ssh-parallel <n>` | Inventory hosts scanned at once (1-16, default 1). |
| `--ssh-insecure-ignore-host-key` | Disable host-key verification. Prints a loud warning every time; lab use only. |

`--target`, `--level`, `--os-family`, `--include-manual`, `--format`, `--output`, `--push-url`,
`--plugin` and `--ai-analyze` work as for a local scan. `--os-family` overrides the remote
detection. `--auto-detect` selects the remote OS's rules only: installed software is not probed
remotely in this phase, so name software targets (`--target nginx`) explicitly. `--device-config`
cannot be combined with `--ssh`.

### Inventory file

```text
# [user@]host[:port] [target]
auditor@web01                     # target: --target (default "all": the host's OS rules)
auditor@web02:2222  linux
netops@core1        cisco-ios
netops@mx1          juniper-junos
```

With an inventory, machine-readable formats need `--output <directory>`: each host gets
`<host>[_<port>].<format>` there (`.txt` for `-f text -o dir`). Text format without `--output`
prints each host's summary to stdout. Hosts that cannot be reached are listed in the summary and
make the command exit non-zero; the other hosts' reports are still written.

## Security model

- **Host keys are always verified** against known_hosts. An unknown host fails with its SHA256
  fingerprint and the command to add it (`ssh-keyscan -p <port> <host> >> ~/.ssh/known_hosts`);
  verify that fingerprint out of band first. A changed key fails with `HOST KEY MISMATCH` and
  nothing is sent to the host. `--ssh-insecure-ignore-host-key` exists for isolated labs and
  always prints a warning.
- **Authentication** (in this order): ssh-agent, the key file (an encrypted key's passphrase is
  asked for on a terminal only), then password / keyboard-interactive, asked for on a terminal or
  taken from `TMAEGIS_SSH_PASSWORD`. The environment variable is **discouraged**: it leaks through
  process listings, shell history, crash dumps and CI logs; tmaegis prints a warning when it is
  set. Prefer ssh-agent or a dedicated key.
- **Nothing is stored**: no passwords, passphrases, keys or host keys are written anywhere, and
  secrets never appear in logs or error messages. tmaegis never edits known_hosts.
- **SSH only, never in the clear**: there is no Telnet, rsh or plain-text fallback.
- **One connection per host**, with at most 4 concurrent sessions on it, a 2-minute timeout per
  command and a 16 MiB cap on each command's output (the session is closed when exceeded).
- **Shell hosts** run the rules' own audit commands (the same read-only commands a local scan
  runs), wrapped as `sh -c '<command>'` (`bash -c` or `powershell -EncodedCommand` where the rule
  says so). Files are read with `cat`; `requires_file` uses `test -e`. Use an unprivileged audit
  account where the benchmark allows it: some checks need root to read protected files.
- **Network devices** receive only commands that pass an allow-list: a fixed list of session
  settings (`terminal length 0`, `terminal pager 0`, `no page`, `set cli pager off`,
  `set cli config-output-format set`, `set cli screen-length 0`, `set clienv rows 0`) or a
  command starting with `show`, `display`, `get`, `export` or `cat` with no shell control
  characters, only output filters after `|` (`display set`, `no-more`, `include`, ...) and none
  of the write/control words (`configure`, `conf`, `write`, `commit`, `reload`, `delete`,
  `erase`, `copy`, `clear`, `set`, `request`, `execute`, `enable`, `debug`, ...). This applies to
  the built-in table and to `--ssh-command`. tmaegis never sends `enable`; use an account that
  lands at the privilege needed to show the configuration.
- Interactive CLIs are driven through a pty: tmaegis waits for the prompt, sends each command,
  answers `--More--`-style pagers with a space, strips ANSI escapes, the echo and the prompt, and
  hands the text to the `config_regex` rules.

## Vendor command table

`Verified` means the command is known to print the format the rules in `rules/network/<target>`
are written for. Unverified vendors **require** `--ssh-command`; the listed command is only a
suggestion that you must confirm on your platform and release.

| Target | Platform | Mode | Commands sent | Verified |
| --- | --- | --- | --- | --- |
| `cisco-ios` | Cisco IOS / IOS XE | shell | `terminal length 0`, `show running-config` | yes |
| `cisco-nxos` | Cisco NX-OS | shell | `terminal length 0`, `show running-config` | yes |
| `cisco-iosxr` | Cisco IOS XR | shell | `terminal length 0`, `show running-config` | yes |
| `cisco-asa` | Cisco ASA | shell | `terminal pager 0`, `show running-config` | yes |
| `juniper-junos` | Juniper Junos OS | shell | `show configuration \| display set \| no-more` | yes |
| `arista-eos` | Arista EOS | shell | `terminal length 0`, `show running-config` | yes |
| `aruba-cx` | HPE Aruba AOS-CX | shell | `no page`, `show running-config` | yes |
| `checkpoint` | Check Point Gaia (clish) | shell | `show configuration` | yes |
| `fortigate` | Fortinet FortiOS | shell | `show full-configuration` | yes |
| `cisco-ftd` | Cisco FTD | shell | suggested: `show running-config` | no |
| `extreme-slx` | Extreme SLX-OS | shell | suggested: `terminal length 0`, `show running-config` | no |
| `fortiswitch` | FortiSwitchOS | shell | suggested: `show full-configuration` | no |
| `paloalto` | PAN-OS | shell | suggested: `set cli pager off`, `set cli config-output-format set`, `show config running` (rules expect set format) | no |
| `f5-bigip` | F5 BIG-IP (tmsh) | shell | suggested: `show running-config` from a tmsh login shell | no |
| `pfsense` | pfSense | exec | suggested: `cat /conf/config.xml` (the admin account opens the console menu) | no |
| `opnsense` | OPNsense | exec | suggested: `cat /conf/config.xml` (root opens the console menu) | no |
| `sophos` | Sophos Firewall | - | none: no `config_regex` rules exist for Sophos yet | no |

Platform notes: Junos needs a login class that lands in the Junos CLI (root lands in a Unix
shell). Check Point needs clish as the login shell, not expert/bash. ASA needs an account that
lands in privileged EXEC. Pager setting commands are only sent where they are known to exist;
elsewhere the pager is answered automatically.

## Limits (phase 1)

- No remote `apply` / remediation of any kind.
- No mobile devices (iPhone/iPad, Android): they have no SSH audit surface; use the MDM rules.
- Installed software is not detected remotely (`--auto-detect` uses the remote OS only).
- `cloud_check` rules are skipped in a remote host scan (they audit cloud APIs, not the host).
- Windows hosts work only through OpenSSH for Windows with PowerShell available; Windows over
  SSH is lightly tested.
- The interactive CLI driver recognises common prompts (`R1#`, `user@mx> `, `FGT # `,
  `RP/0/RSP0/CPU0:xr#`, tmsh prompts); unusual prompts may need a standard login banner/prompt.
