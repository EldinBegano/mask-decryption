// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <file.mlp>",
		Short: "Check a .mlp file's integrity without writing decrypted output",
		Long: `Decrypt a .mlp file in memory just far enough to check its
authentication tag, without ever writing decrypted output to disk.

Requires the keyfile (unlike 'mlp info', which only reads the header and
needs no key). Exits 6 if the file fails authentication — tampered,
corrupted, or encrypted under a different key — otherwise prints "OK" and
exits 0.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(args[0])
		},
	}
}

func runVerify(inputPath string) error {
	if !strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is not a .mlp file", inputPath))
	}

	hdr, headerBytes, ciphertext, err := ops.ReadMLP(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	key, err := keystore.LoadKey()
	if err != nil {
		return withCode(2, err)
	}

	_, legacy, err := ops.OpenPayload(key, hdr, headerBytes, ciphertext)
	if err != nil {
		return withCode(6, fmt.Errorf("%s: %w", inputPath, err))
	}

	fmt.Printf("OK: %s is intact\n", inputPath)
	if legacy {
		warnLegacyHeader(inputPath)
	}
	return nil
}
