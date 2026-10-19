// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// Mode is how commands reach a device.
type Mode string

const (
	// ModeShell drives an interactive CLI (pty + shell) with prompt detection and pager
	// handling; most network operating systems have no usable exec channel.
	ModeShell Mode = "shell"
	// ModeExec runs each command on its own exec channel (BSD-based firewalls).
	ModeExec Mode = "exec"
)

// Vendor describes how to fetch a device configuration read-only.
type Vendor struct {
	Target string
	Name   string
	Mode   Mode
	// Setup commands only change the CLI session (pager off); their output is discarded.
	Setup []string
	// Commands produce the configuration text the rules are evaluated against.
	Commands []string
	// Verified is true only when the commands are known to produce the format the rules in
	// rules/network/<target> are written for. Unverified vendors require --ssh-command;
	// Commands then only serve as a documented suggestion.
	Verified bool
	Note     string
}

// Vendors is the read-only command table, keyed by rule target.
var Vendors = map[string]Vendor{
	"cisco-ios": {Target: "cisco-ios", Name: "Cisco IOS / IOS XE", Mode: ModeShell,
		Setup: []string{"terminal length 0"}, Commands: []string{"show running-config"}, Verified: true,
		Note: "log in at privilege 15 (or with an account authorised for show running-config)"},
	"cisco-nxos": {Target: "cisco-nxos", Name: "Cisco NX-OS", Mode: ModeShell,
		Setup: []string{"terminal length 0"}, Commands: []string{"show running-config"}, Verified: true},
	"cisco-iosxr": {Target: "cisco-iosxr", Name: "Cisco IOS XR", Mode: ModeShell,
		Setup: []string{"terminal length 0"}, Commands: []string{"show running-config"}, Verified: true},
	"cisco-asa": {Target: "cisco-asa", Name: "Cisco ASA", Mode: ModeShell,
		Setup: []string{"terminal pager 0"}, Commands: []string{"show running-config"}, Verified: true,
		Note: "the account must land in privileged EXEC (aaa authorization exec / privilege 15); tmaegis never sends enable"},
	"cisco-ftd": {Target: "cisco-ftd", Name: "Cisco Secure Firewall Threat Defense (FTD)", Mode: ModeShell,
		Commands: []string{"show running-config"},
		Note:     "FTD CLI behaviour differs between versions and FMC/FDM management; confirm on your release"},
	"juniper-junos": {Target: "juniper-junos", Name: "Juniper Junos OS", Mode: ModeShell,
		Commands: []string{"show configuration | display set | no-more"}, Verified: true,
		Note: "use a non-root login class that lands in the Junos CLI (root lands in a Unix shell)"},
	"arista-eos": {Target: "arista-eos", Name: "Arista EOS", Mode: ModeShell,
		Setup: []string{"terminal length 0"}, Commands: []string{"show running-config"}, Verified: true},
	"aruba-cx": {Target: "aruba-cx", Name: "HPE Aruba Networking AOS-CX", Mode: ModeShell,
		Setup: []string{"no page"}, Commands: []string{"show running-config"}, Verified: true},
	"extreme-slx": {Target: "extreme-slx", Name: "Extreme SLX-OS", Mode: ModeShell,
		Setup: []string{"terminal length 0"}, Commands: []string{"show running-config"}},
	"checkpoint": {Target: "checkpoint", Name: "Check Point Gaia (clish)", Mode: ModeShell,
		Commands: []string{"show configuration"}, Verified: true,
		Note: "the account's shell must be clish, not expert/bash"},
	"fortigate": {Target: "fortigate", Name: "Fortinet FortiGate (FortiOS)", Mode: ModeShell,
		Commands: []string{"show full-configuration"}, Verified: true,
		Note: "on multi-VDOM units the global and per-VDOM sections appear in one dump"},
	"fortiswitch": {Target: "fortiswitch", Name: "Fortinet FortiSwitchOS", Mode: ModeShell,
		Commands: []string{"show full-configuration"}},
	"paloalto": {Target: "paloalto", Name: "Palo Alto Networks PAN-OS", Mode: ModeShell,
		Setup:    []string{"set cli pager off", "set cli config-output-format set"},
		Commands: []string{"show config running"},
		Note:     "the rules expect set-format output; whether operational-mode 'show config running' honours config-output-format varies by release"},
	"f5-bigip": {Target: "f5-bigip", Name: "F5 BIG-IP (tmsh)", Mode: ModeShell,
		Commands: []string{"show running-config"},
		Note:     "needs a tmsh login shell; the rules expect tmsh list-style blocks"},
	"pfsense": {Target: "pfsense", Name: "Netgate pfSense", Mode: ModeExec,
		Commands: []string{"cat /conf/config.xml"},
		Note:     "the admin account opens the console menu; use an account with a shell that may read /conf/config.xml"},
	"opnsense": {Target: "opnsense", Name: "OPNsense", Mode: ModeExec,
		Commands: []string{"cat /conf/config.xml"},
		Note:     "root opens the console menu; use an account with a shell that may read /conf/config.xml"},
	"sophos": {Target: "sophos", Name: "Sophos Firewall", Mode: ModeShell,
		Note: "no config_regex rules exist for Sophos yet; nothing is evaluated over SSH"},
}

// VendorTargets lists the targets in the table, sorted.
func VendorTargets() []string {
	var out []string
	for t := range Vendors {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// IsDevice reports whether target is a network device fetched with the vendor table.
func IsDevice(target string) bool { _, ok := Vendors[strings.ToLower(target)]; return ok }

// Plan is what will actually be sent to a device.
type Plan struct {
	Vendor   Vendor
	Setup    []string
	Commands []string
}

// PlanFor chooses the commands for target: the user's explicit commands (all of which become
// fetch commands, except recognised session settings), otherwise the table entry if it is
// Verified. Every command is checked against the read-only allow-list.
func PlanFor(target string, userCommands []string) (Plan, error) {
	v, ok := Vendors[strings.ToLower(target)]
	if !ok {
		return Plan{}, fmt.Errorf("no SSH command table entry for target %q (devices: %s)", target, strings.Join(VendorTargets(), ", "))
	}
	p := Plan{Vendor: v}
	if len(userCommands) > 0 {
		for _, c := range userCommands {
			c = strings.TrimSpace(c)
			if err := CheckCommand(c); err != nil {
				return Plan{}, err
			}
			if sessionSettings[strings.Join(strings.Fields(strings.ToLower(c)), " ")] {
				p.Setup = append(p.Setup, c)
			} else {
				p.Commands = append(p.Commands, c)
			}
		}
		if len(p.Commands) == 0 {
			return Plan{}, fmt.Errorf("%w: --ssh-command only set session options; add the command that prints the configuration", ErrNotAllowed)
		}
		return p, nil
	}
	if !v.Verified {
		hint := "no command is known"
		if all := append(append([]string{}, v.Setup...), v.Commands...); len(v.Commands) > 0 {
			hint = "a likely but unverified sequence is: --ssh-command '" + strings.Join(all, "' --ssh-command '") + "'"
		}
		return Plan{}, fmt.Errorf("the SSH command for %s (%s) is not verified; pass --ssh-command explicitly (%s). See docs/REMOTE.md", v.Name, v.Target, hint)
	}
	p.Setup, p.Commands = v.Setup, v.Commands
	for _, c := range append(append([]string{}, p.Setup...), p.Commands...) {
		if err := CheckCommand(c); err != nil {
			return Plan{}, err // the table itself must obey the guard
		}
	}
	return p, nil
}

// FetchConfig runs the plan on the device and returns the concatenated output of its
// fetch commands.
func (c *Client) FetchConfig(ctx context.Context, p Plan) ([]byte, error) {
	if p.Vendor.Mode == ModeExec {
		var out []byte
		for _, cmd := range p.Commands {
			if err := CheckCommand(cmd); err != nil {
				return nil, err
			}
			b, err := c.Exec(ctx, cmd)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", cmd, err)
			}
			out = append(out, b...)
		}
		return out, nil
	}
	sh, err := c.OpenShell(ctx)
	if err != nil {
		return nil, err
	}
	defer sh.Close()
	for _, cmd := range p.Setup {
		if _, err := sh.Run(ctx, cmd); err != nil {
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
	}
	var out []byte
	for _, cmd := range p.Commands {
		b, err := sh.Run(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
		out = append(out, b...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
	}
	return out, nil
}

// DeviceExecutor stands in for a network device during evaluation: rules only read the
// captured configuration, so it refuses to run anything and reports no files. It exists so
// the report names the device and remediation is refused.
type DeviceExecutor struct{ Info engine.Target }

var _ engine.Remote = DeviceExecutor{}

func (DeviceExecutor) Run(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("%w: device rules are evaluated against the captured configuration only", ErrNotAllowed)
}

func (DeviceExecutor) ReadFile(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("%w: device rules are evaluated against the captured configuration only", ErrNotAllowed)
}

func (DeviceExecutor) Exists(context.Context, string) bool { return false }

func (d DeviceExecutor) Target() engine.Target { return d.Info }
