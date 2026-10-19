// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/discovery"
)

var discoverJSON bool

var discoverCmd = &cobra.Command{
	Use: "discover",
	RunE: func(cmd *cobra.Command, args []string) error {
		host := discovery.Detect()
		if discoverJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(host)
		}
		fmt.Println(i18n.T("cli.discover.os", "value", host.OS))
		fmt.Println(i18n.T("cli.discover.distro", "value", host.Distro+" "+host.Version))
		fmt.Println(i18n.T("cli.discover.family", "value", host.Family))
		fmt.Println(i18n.T("cli.discover.software", "value", strings.Join(host.Software, ", ")))
		return nil
	},
}

func init() {
	discoverCmd.Flags().BoolVar(&discoverJSON, "json", false, "")
	rootCmd.AddCommand(discoverCmd)
}
