// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"fmt"
	"strings"
)

// readVerbs are the only words a device command may start with.
var readVerbs = map[string]bool{"show": true, "display": true, "get": true, "export": true, "cat": true}

// sessionSettings are the exact pager/format commands that only change the current CLI
// session (never the device configuration). Matched case-insensitively after collapsing spaces.
var sessionSettings = map[string]bool{
	"terminal length 0":                true, // Cisco IOS/NX-OS/IOS-XR, Arista EOS, Extreme SLX
	"terminal pager 0":                 true, // Cisco ASA
	"no page":                          true, // Aruba AOS-CX
	"set cli pager off":                true, // Palo Alto PAN-OS operational mode
	"set cli config-output-format set": true, // Palo Alto PAN-OS operational mode
	"set cli screen-length 0":          true, // Juniper Junos operational mode
	"set clienv rows 0":                true, // Check Point Gaia clish
}

// forbidden words never appear anywhere in a device command (outside a match pattern).
var forbidden = map[string]bool{
	"configure": true, "conf": true, "write": true, "wr": true, "commit": true,
	"reload": true, "reboot": true, "restart": true, "shutdown": true, "halt": true, "poweroff": true,
	"delete": true, "del": true, "erase": true, "format": true, "rm": true, "mv": true, "cp": true,
	"copy": true, "clear": true, "set": true, "unset": true, "edit": true, "execute": true, "exec": true,
	"request": true, "rollback": true, "load": true, "save": true, "install": true, "upgrade": true,
	"debug": true, "undebug": true, "kill": true, "start": true, "stop": true, "tee": true, "dd": true,
	"modify": true, "create": true, "add": true, "remove": true, "purge": true, "factory-reset": true,
	"reset": true, "test": true, "ping": true, "traceroute": true, "telnet": true, "ssh": true,
	"scp": true, "tftp": true, "ftp": true, "sudo": true, "su": true, "enable": true, "expert": true,
}

// pipeFilters may follow "|" in a device command (output filters only).
var pipeFilters = map[string]bool{
	"display": true, "no-more": true, "match": true, "include": true, "exclude": true, "except": true,
	"begin": true, "section": true, "count": true, "grep": true, "find": true, "last": true, "trim": true,
}

// displayArgs may follow "| display" (Junos output formats).
var displayArgs = map[string]bool{"set": true, "xml": true, "json": true, "inheritance": true, "omit": true}

// CheckCommand enforces the read-only allow-list for commands sent to network devices: a
// known session/pager setting, or a read verb (show/display/get/export/cat) with no forbidden
// word, no shell control characters, and only output filters after "|".
func CheckCommand(cmd string) error {
	norm := strings.Join(strings.Fields(strings.ToLower(cmd)), " ")
	if norm == "" {
		return fmt.Errorf("%w: empty command", ErrNotAllowed)
	}
	if sessionSettings[norm] {
		return nil
	}
	if strings.ContainsAny(cmd, ";&<>`$\n\r\\") {
		return fmt.Errorf("%w: %q contains shell control characters", ErrNotAllowed, cmd)
	}
	segments := strings.Split(norm, "|")
	first := strings.Fields(segments[0])
	if len(first) == 0 || !readVerbs[first[0]] {
		return fmt.Errorf("%w: %q must start with one of show, display, get, export or cat", ErrNotAllowed, cmd)
	}
	if err := noForbidden(cmd, first); err != nil {
		return err
	}
	for _, seg := range segments[1:] {
		words := strings.Fields(seg)
		if len(words) == 0 || !pipeFilters[words[0]] {
			return fmt.Errorf("%w: %q pipes into something other than an output filter", ErrNotAllowed, cmd)
		}
		switch words[0] {
		case "display":
			for _, w := range words[1:] {
				if !displayArgs[w] {
					return fmt.Errorf("%w: %q uses an unknown display option %q", ErrNotAllowed, cmd, w)
				}
			}
		case "match", "include", "exclude", "except", "begin", "section", "grep", "find":
			// The rest is a search pattern, not a command.
		default:
			if err := noForbidden(cmd, words[1:]); err != nil {
				return err
			}
		}
	}
	return nil
}

func noForbidden(cmd string, words []string) error {
	for _, w := range words {
		w = strings.Trim(w, `"'`)
		if forbidden[w] || strings.HasPrefix(w, "write") || strings.HasPrefix(w, "commit") ||
			strings.HasPrefix(w, "reload") || strings.HasPrefix(w, "erase") || strings.HasPrefix(w, "delete") {
			return fmt.Errorf("%w: %q contains the write/control word %q", ErrNotAllowed, cmd, w)
		}
	}
	return nil
}
