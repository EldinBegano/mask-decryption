// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/keystore"
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "verify <file.mlp>",
		Short:         "Check a .mlp file's integrity without writing decrypted output",
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

	if _, err := crypto.Decrypt(key, hdr.Nonce[:], ciphertext, hdr.AAD(headerBytes)); err != nil {
		return withCode(6, fmt.Errorf("%s: %w", inputPath, err))
	}

	fmt.Printf("OK: %s is intact\n", inputPath)
	return nil
}
