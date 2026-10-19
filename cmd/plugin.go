// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/plugin"
)

var pluginDir string

var pluginCmd = &cobra.Command{
	Use: "plugin",
}

var pluginListCmd = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := plugin.NewManager(pluginDir)
		plugins, err := pm.Discover()
		if err != nil {
			return err
		}
		fmt.Println(i18n.T("cli.plugin.listHeader"))
		if len(plugins) == 0 {
			fmt.Println(i18n.T("cli.plugin.none", "dir", pluginDir))
			return nil
		}
		for _, p := range plugins {
			fmt.Printf("  - %s (%s) [%s]\n", p.Name, p.Type, p.Path)
		}
		return nil
	},
}

var installSHA string

var pluginInstallCmd = &cobra.Command{
	Use:  "install <source>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := plugin.NewManager(pluginDir)
		path, sum, err := pm.Install(cmd.Context(), args[0], plugin.InstallOptions{SHA256: installSHA})
		if err != nil {
			return err
		}
		fmt.Println(i18n.T("cli.plugin.installed", "path", path))
		fmt.Printf("    sha256: %s\n", sum)
		fmt.Println(i18n.T("cli.plugin.trustWarning"))
		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:  "remove <name>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return plugin.NewManager(pluginDir).Remove(args[0])
	},
}

func init() {
	pluginInstallCmd.Flags().StringVar(&installSHA, "sha256", "", "")
	pluginCmd.AddCommand(pluginInstallCmd, pluginRemoveCmd)
	pluginCmd.PersistentFlags().StringVar(&pluginDir, "plugin-dir", "./plugins", "")
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}
