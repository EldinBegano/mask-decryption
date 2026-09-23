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
		Short:         "Generate a new keyfile (the old one is kept as keyfile.old; files under it need it restored to decrypt)",
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
		ok := confirm("A keyfile already exists. Regenerating it means .mlp files encrypted with the old key can no longer be decrypted unless you restore it (the old key is kept as keyfile.old). Continue?")
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	hadOldKey := exists
	if _, err := keystore.Keygen(); err != nil {
		return withCode(1, err)
	}

	dir, err := keystore.ConfigDir()
	if err != nil {
		return withCode(1, err)
	}
	fmt.Printf("new keyfile created at %s\n", filepath.Join(dir, "keyfile"))
	if hadOldKey {
		oldPath, _ := keystore.OldKeyPath()
		fmt.Printf("previous key kept at %s — delete it once you no longer need it\n", oldPath)
	}
	fmt.Fprintln(os.Stderr, "WARNING: back it up now with 'mlp keyfile export <path>' — if it's lost, encrypted files become permanently unreadable.")
	return nil
}
