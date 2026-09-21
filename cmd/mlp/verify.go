// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/fileformat"
	"github.com/EldinBegano/mask-decryption/internal/keystore"
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

	f, err := os.Open(inputPath)
	if err != nil {
		return withCode(1, err)
	}
	defer f.Close()

	hdr, err := fileformat.ReadHeader(f)
	if err != nil {
		return withCode(1, fmt.Errorf("%s: %w", inputPath, err))
	}
	ciphertext, err := io.ReadAll(f)
	if err != nil {
		return withCode(1, err)
	}

	key, err := keystore.LoadKey()
	if err != nil {
		return withCode(2, err)
	}

	if _, err := crypto.Decrypt(key, hdr.Nonce[:], ciphertext); err != nil {
		return withCode(6, fmt.Errorf("%s: %w", inputPath, err))
	}

	fmt.Printf("OK: %s is intact\n", inputPath)
	return nil
}
