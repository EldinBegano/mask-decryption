// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

func newKeyfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "keyfile",
		Short:         "Back up or restore the keyfile",
		Long:          `Back up or restore the keyfile and its nonce counter. See the export and import subcommands.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(newKeyfileExportCmd(), newKeyfileImportCmd())
	return cmd
}

func newKeyfileExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <path>",
		Short: "Copy the keyfile and counter state to <path> for backup",
		Long: `Copy the keyfile and its nonce counter into <path> (created if needed),
bundled together so a later import continues the counter correctly instead
of risking nonce reuse. Prints the exported keyfile's SHA-256 so you can
verify a copy (e.g. onto a USB stick) matches.

This is the only way to protect against keyfile loss: there is no other
recovery mechanism. Run it right after the first encrypt creates a
keyfile, and again after any 'mlp keygen' or 'mlp rotate'.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyfileExport(args[0])
		},
	}
}

func runKeyfileExport(dest string) error {
	if err := keystore.Export(dest); err != nil {
		if errors.Is(err, keystore.ErrKeyfileMissing) {
			return withCode(2, err)
		}
		return withCode(1, err)
	}
	sum, err := keystore.KeyfileSHA256()
	if err != nil {
		return withCode(1, err)
	}
	fmt.Printf("exported keyfile -> %s\n", dest)
	fmt.Printf("sha256: %s\n", sum)
	return nil
}

func newKeyfileImportCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Restore a keyfile and counter state backup from <path>",
		Long: `Install a keyfile+counter backup from <path> (as written by 'mlp keyfile
export') into the config directory, making it the active key.

If a keyfile already exists, it's kept as keyfile.old (with its matching
counter as counter.old) first — restoring the wrong backup by mistake is
then recoverable by hand, not permanent.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyfileImport(args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runKeyfileImport(src string, yes bool) error {
	exists, err := keystore.Exists()
	if err != nil {
		return withCode(1, err)
	}
	if exists && !yes {
		ok := confirm("A keyfile already exists. Importing replaces it and its nonce counter — files encrypted with the current key can no longer be decrypted unless you restore it (it's kept as keyfile.old, with counter.old, if you pointed this at the wrong backup by mistake). Continue?")
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	hadOldKey := exists
	if err := keystore.Import(src); err != nil {
		return withCode(1, err)
	}
	fmt.Println("keyfile imported")
	if hadOldKey {
		oldPath, _ := keystore.OldKeyPath()
		fmt.Printf("previous key kept at %s — delete it once you no longer need it\n", oldPath)
	}
	return nil
}

func confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
