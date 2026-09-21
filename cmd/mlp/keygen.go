// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

func newKeygenCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:           "keygen",
		Short:         "Generate a new keyfile (files encrypted under the old key become permanently unreadable)",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeygen(yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runKeygen(yes bool) error {
	exists, err := keystore.Exists()
	if err != nil {
		return withCode(1, err)
	}
	if exists && !yes {
		ok := confirm("A keyfile already exists. Regenerating it makes every .mlp file encrypted with the old key permanently unreadable. Continue?")
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	if _, err := keystore.Keygen(); err != nil {
		return withCode(1, err)
	}

	dir, err := keystore.ConfigDir()
	if err != nil {
		return withCode(1, err)
	}
	fmt.Printf("new keyfile created at %s\n", filepath.Join(dir, "keyfile"))
	fmt.Fprintln(os.Stderr, "WARNING: back it up now with 'mlp keyfile export <path>' — if it's lost, encrypted files become permanently unreadable.")
	return nil
}
