# Sample device configurations

Small, synthetic network-device configuration files so you can try the device-config audit
without owning a device. Each target has a `-compliant` and a `-noncompliant` variant.

These files are **synthetic and are not secure templates**: hostnames and addresses use
documentation ranges (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24, example.com), secrets are
placeholders (`REDACTED`, fake hashes), and the "compliant" files only aim to satisfy the
automated regex rules of this repository, not to be a complete or safe production
configuration. Do not deploy them.

## Usage

```
tmaegis scan --target <id> --device-config samples/network/<id>/<id>-compliant.txt
tmaegis scan --target <id> --device-config samples/network/<id>/<id>-noncompliant.txt
```

Run from the repository root (the rules are read from `./rules`) with the default level 1.
With `--device-config`, only the `config_regex` rules of the target are evaluated.

## Results

Counts measured with the default level (`--level 1`) and `--device-config`.

| Target | Files | Rules | Compliant PASS/FAIL | Non-compliant PASS/FAIL |
|---|---|---|---|---|
| arista-eos | `network/arista-eos/` | 22 | 21 / 1 | 2 / 20 |
| aruba-cx | `network/aruba-cx/` | 22 | 22 / 0 | 1 / 21 |
| checkpoint | `network/checkpoint/` | 34 | 34 / 0 | 3 / 31 |
| cisco-asa | `network/cisco-asa/` | 61 | 61 / 0 | 3 / 58 |
| cisco-ftd | `network/cisco-ftd/` | 3 | 3 / 0 | 0 / 3 |
| cisco-ios | `network/cisco-ios/` | 63 | 63 / 0 | 4 / 59 |
| cisco-iosxr | `network/cisco-iosxr/` | 30 | 30 / 0 | 3 / 27 |
| cisco-nxos | `network/cisco-nxos/` | 30 | 30 / 0 | 4 / 26 |
| extreme-slx | `network/extreme-slx/` | 15 | 15 / 0 | 1 / 14 |
| f5-bigip | `network/f5-bigip/` | 20 | 20 / 0 | 0 / 20 |
| fortigate | `network/fortigate/` | 33 | 33 / 0 | 0 / 33 |
| fortiswitch | `network/fortiswitch/` | 25 | 25 / 0 | 0 / 25 |
| juniper-junos | `network/juniper-junos/` | 104 | 104 / 0 | 31 / 73 |
| opnsense | `network/opnsense/` | 15 | 15 / 0 | 0 / 15 |
| paloalto | `network/paloalto/` | 48 | 48 / 0 | 3 / 45 |
| pfsense | `network/pfsense/` | 20 | 20 / 0 | 0 / 20 |
| sophos | `network/sophos/` | 0 | 0 / 0 | 0 / 0 |

Notes:

- `sophos`: all Sophos rules are manual-review checks (no `config_regex` rules), so a device-config
  scan evaluates 0 rules. The two files are illustrative only.
- `arista-eos`: rule 2.6.1 expects a bare `shutdown` inside `management ssh`, which would
  disable SSH, so the compliant sample leaves it failing.
- `cisco-ftd` has only 3 device-config rules at level 1.
- Rules were not modified. `manifest.json` lists the same data (paths relative to `samples/`),
  and `go test ./samples` checks that each compliant file passes more rules than its non-compliant one.
