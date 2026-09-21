// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/fileformat"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <file.mlp>",
		Short: "Show a .mlp file's header without decrypting it",
		Long: `Show what a .mlp file says about itself: format version, original
extension, and the name it would decrypt to. Reads only the header, so it
works without a keyfile and cannot tell whether the file is intact — use
'mlp verify' for that.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(args[0])
		},
	}
}

func runInfo(inputPath string) error {
	if !strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is not a .mlp file", inputPath))
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return withCode(1, err)
	}
	if info.IsDir() {
		return withCode(1, fmt.Errorf("%s is a directory", inputPath))
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

	payload := info.Size() - int64(hdr.Size())
	if payload < crypto.TagSize {
		return withCode(1, fmt.Errorf("%s: truncated (no room for the authentication tag)", inputPath))
	}

	ext := hdr.Ext
	if ext == "" {
		ext = "(none)"
	}
	fmt.Printf("file:            %s\n", inputPath)
	fmt.Printf("format version:  %d\n", fileformat.Version)
	fmt.Printf("original ext:    %s\n", ext)
	fmt.Printf("decrypts to:     %s\n", restoreExt(strings.TrimSuffix(inputPath, ".mlp"), hdr.Ext))
	fmt.Printf("plaintext size:  %d bytes\n", payload-crypto.TagSize)
	return nil
}
