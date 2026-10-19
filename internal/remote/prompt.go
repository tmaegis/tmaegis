// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// promptMu serialises terminal prompts when several hosts connect in parallel.
var promptMu sync.Mutex

// ErrNoTerminal is returned when a secret is needed but stdin is not a terminal.
var ErrNoTerminal = errors.New("a passphrase or password is required but stdin is not a terminal (use ssh-agent, or " + PasswordEnv + " as a discouraged last resort)")

// TerminalPrompt reads a secret from the terminal with echo disabled. The prompt goes to
// stderr; the answer is returned to the caller only and never echoed, logged or stored.
func TerminalPrompt(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", ErrNoTerminal
	}
	promptMu.Lock()
	defer promptMu.Unlock()
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
