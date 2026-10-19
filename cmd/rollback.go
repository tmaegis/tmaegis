// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/snapshot"
)

var (
	snapshotID    string
	listSnapshots bool
)

var rollbackCmd = &cobra.Command{
	Use: "rollback",
	RunE: func(cmd *cobra.Command, args []string) error {
		snapMgr := snapshot.NewManager(cfg.Engine.SnapshotDir)
		if listSnapshots {
			infos, err := snapMgr.List()
			if err != nil {
				return err
			}
			if len(infos) == 0 {
				fmt.Println(i18n.T("cli.rollback.none", "dir", cfg.Engine.SnapshotDir))
			}
			for _, i := range infos {
				fmt.Println(i18n.N("cli.rollback.entry", i.Files, "id", i.ID, "created", i.CreatedAt.Format("2006-01-02 15:04:05 MST")))
			}
			return nil
		}
		if snapshotID == "" {
			latest, err := snapMgr.GetLatestSnapshotID()
			if err != nil {
				return lerr.Wrap("cli.rollback.err.notFound", err)
			}
			snapshotID = latest
		}
		fmt.Println(i18n.T("cli.rollback.rolling", "id", snapshotID))
		if err := snapMgr.Rollback(snapshotID); err != nil {
			return lerr.Wrap("cli.rollback.err.failed", err)
		}
		fmt.Println(i18n.T("cli.rollback.restored"))
		return nil
	},
}

func init() {
	rollbackCmd.Flags().BoolVar(&listSnapshots, "list", false, "")
	rollbackCmd.Flags().StringVar(&snapshotID, "id", "", "")
	rootCmd.AddCommand(rollbackCmd)
}
