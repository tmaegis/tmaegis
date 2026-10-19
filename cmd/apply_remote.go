// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/lerr"
)

// apply accepts --ssh / --ssh-inventory only to explain that remote remediation does not
// exist: remote support is read-only in this release.
func init() {
	applyCmd.Flags().String("ssh", "", "")
	applyCmd.Flags().String("ssh-inventory", "", "")
	_ = applyCmd.Flags().MarkHidden("ssh")
	_ = applyCmd.Flags().MarkHidden("ssh-inventory")
	prev := applyCmd.PreRunE
	applyCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("ssh") || cmd.Flags().Changed("ssh-inventory") {
			return lerr.New("cli.apply.err.ssh")
		}
		if prev != nil {
			return prev(cmd, args)
		}
		return nil
	}
}
