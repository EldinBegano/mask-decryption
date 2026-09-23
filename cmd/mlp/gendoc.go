// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newGendocCmd generates man pages for every command into a directory.
// Hidden — this is a build-time tool (see .goreleaser.yaml's before hooks),
// not something an end user runs.
func newGendocCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "gendoc <dir>",
		Short:         "Generate man pages into <dir> (build-time use only)",
		Hidden:        true,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			header := &doc.GenManHeader{
				Title:   "MLP",
				Section: "1",
				Source:  "mlp " + version,
			}
			if err := doc.GenManTree(cmd.Root(), header, dir); err != nil {
				return err
			}
			fmt.Println("generated man pages ->", dir)
			return nil
		},
	}
}
