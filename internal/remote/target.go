// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package remote audits other machines over SSH, strictly read-only. It provides an
// engine.Executor that runs the same rules on a remote shell host, and fetches network-device
// configurations with a fixed table of read-only vendor commands.
package remote

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// Endpoint is one SSH destination: [user@]host[:port].
type Endpoint struct {
	User string
	Host string
	Port int
}

// Addr is host:port, with IPv6 literals bracketed.
func (e Endpoint) Addr() string { return net.JoinHostPort(e.Host, strconv.Itoa(e.Port)) }

// String renders the endpoint as user@host[:port] (port omitted when 22).
func (e Endpoint) String() string {
	h := e.Host
	if strings.Contains(h, ":") {
		h = "[" + h + "]"
	}
	if e.Port != 22 {
		h += ":" + strconv.Itoa(e.Port)
	}
	if e.User != "" {
		return e.User + "@" + h
	}
	return h
}

// ParseEndpoint parses [user@]host[:port]; IPv6 literals need brackets when a port is given
// ([::1]:2222). defaultUser is used when no user is present.
func ParseEndpoint(s, defaultUser string) (Endpoint, error) {
	s = strings.TrimSpace(s)
	e := Endpoint{User: defaultUser, Port: 22}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		e.User, s = s[:i], s[i+1:]
	}
	if s == "" {
		return e, fmt.Errorf("missing host")
	}
	switch {
	case strings.HasPrefix(s, "["):
		host, port, err := net.SplitHostPort(s)
		if err != nil {
			if strings.HasSuffix(s, "]") {
				host, port = strings.Trim(s, "[]"), ""
			} else {
				return e, fmt.Errorf("invalid address %q: %v", s, err)
			}
		}
		e.Host = host
		if port != "" {
			p, err := parsePort(port)
			if err != nil {
				return e, err
			}
			e.Port = p
		}
	case strings.Count(s, ":") == 1:
		host, port, _ := strings.Cut(s, ":")
		p, err := parsePort(port)
		if err != nil {
			return e, err
		}
		e.Host, e.Port = host, p
	default:
		e.Host = s // a hostname, IPv4 address or bare IPv6 literal
	}
	if e.Host == "" {
		return e, fmt.Errorf("missing host")
	}
	if e.User == "" {
		return e, fmt.Errorf("no user for %s: use user@host", e.Host)
	}
	if strings.ContainsAny(e.User+e.Host, " \t/\\") {
		return e, fmt.Errorf("invalid destination %q", s)
	}
	return e, nil
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return p, nil
}

// InventoryEntry is one line of an inventory file.
type InventoryEntry struct {
	Endpoint Endpoint
	Target   string // rule target for this host ("" = the --target given on the command line)
	Line     int
}

// ParseInventory reads one "[user@]host[:port] [target]" per line; blank lines and
// everything after # are ignored.
func ParseInventory(r io.Reader, defaultUser string) ([]InventoryEntry, error) {
	var out []InventoryEntry
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) > 2 {
			return nil, fmt.Errorf("line %d: expected \"[user@]host[:port] [target]\"", n)
		}
		ep, err := ParseEndpoint(fields[0], defaultUser)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", n, err)
		}
		e := InventoryEntry{Endpoint: ep, Line: n}
		if len(fields) == 2 {
			e.Target = strings.ToLower(fields[1])
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts in inventory")
	}
	return out, nil
}

// LoadInventory parses the inventory file at path.
func LoadInventory(path, defaultUser string) ([]InventoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseInventory(f, defaultUser)
}
