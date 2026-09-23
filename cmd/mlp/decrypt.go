// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

func newDecryptCmd() *cobra.Command {
	var output string
	var force bool
	cmd := &cobra.Command{
		Use:   "decrypt <file.mlp|dir>",
		Short: "Decrypt a .mlp file (or every .mlp file in a directory, recursively)",
		Long: `Decrypt a .mlp file using the auto-managed keyfile. The original
filename, permission bits, and stored timestamps (if any) are restored.
If the file was compressed at encrypt time, it's transparently
decompressed here.

A tampered or corrupted file fails loudly (auth failure, exit 3) — nothing
is ever written under a header that doesn't match its ciphertext.

Given a directory, every .mlp file under it (recursively) is decrypted the
same way; files that aren't .mlp are ignored. One file failing does not
stop the rest.

Refuses to overwrite an existing output file unless --force is given.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDecrypt(args[0], output, force)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path for a single file (default: original filename)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "replace existing output files instead of failing")
	return cmd
}

func runDecrypt(inputPath, outputPath string, force bool) error {
	opts := ops.Options{Force: force}

	if info, err := os.Stat(inputPath); err == nil && info.IsDir() {
		if outputPath != "" {
			return withCode(1, errors.New("-o/--output cannot be used with a directory"))
		}
		res, err := ops.DecryptDir(keystore.LoadKey, inputPath, opts, batchEvents("decrypted", false))
		if err != nil {
			return opsErr(err)
		}
		return finishBatch("decrypted", res)
	}

	res, err := ops.DecryptFile(keystore.LoadKey, inputPath, outputPath, opts)
	if err != nil {
		return opsErr(err)
	}
	reportDone("decrypted", res, false)
	return nil
}
