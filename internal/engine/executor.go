// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package engine

import (
	"context"
	"os"
)

// Executor runs the read-only parts of an audit: shell commands, file reads and existence
// checks. The default runs them on this machine; internal/remote provides one that runs the
// same rules on another host over SSH.
type Executor interface {
	// Run executes command with the interpreter the rule was written for ("", "sh", "bash" or
	// "powershell") and returns its combined stdout and stderr. A non-zero exit is an error.
	Run(ctx context.Context, command, shell string) ([]byte, error)
	// ReadFile returns the content of path.
	ReadFile(ctx context.Context, path string) ([]byte, error)
	// Exists reports whether path exists.
	Exists(ctx context.Context, path string) bool
}

// Target describes the machine an Executor reaches, for the report header.
type Target struct {
	Hostname string
	OS       string
	Arch     string
}

// Remote is implemented by executors that do not run on this machine. Remediation refuses
// to run through them: remote auditing is read-only.
type Remote interface {
	Executor
	Target() Target
}

// LocalExecutor runs audits on this machine.
type LocalExecutor struct{}

func (LocalExecutor) Run(ctx context.Context, command, shell string) ([]byte, error) {
	return shellCommandWith(ctx, command, shell).CombinedOutput()
}

func (LocalExecutor) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (LocalExecutor) Exists(_ context.Context, path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
