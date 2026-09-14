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

func newDecryptCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:           "decrypt <file.mlp>",
		Short:         "Decrypt a .mlp file",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDecrypt(args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path (default: original filename)")
	return cmd
}

func runDecrypt(inputPath, outputPath string) error {
	if !strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is not a .mlp file", inputPath))
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return withCode(1, err)
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

	plaintext, err := crypto.Decrypt(key, hdr.Nonce[:], ciphertext)
	if err != nil {
		return withCode(3, fmt.Errorf("%s: %w", inputPath, err))
	}

	if outputPath == "" {
		stem := strings.TrimSuffix(inputPath, ".mlp")
		outputPath = restoreExt(stem, hdr.Ext)
	}
	if _, err := os.Stat(outputPath); err == nil {
		return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
	}

	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
		}
		return withCode(1, err)
	}
	defer out.Close()

	if _, err := out.Write(plaintext); err != nil {
		return withCode(1, err)
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return withCode(1, err)
	}

	fmt.Printf("decrypted -> %s\n", outputPath)
	if verbose {
		fmt.Printf("  output: %d bytes\n", len(plaintext))
	}
	return nil
}

// restoreExt appends "."+ext to stem unless stem already carries that
// extension. Handles both the normal case (encrypt appended .mlp to a
// name that already had its extension) and the case where -o dropped it
// during encrypt.
func restoreExt(stem, ext string) string {
	if ext == "" {
		return stem
	}
	if strings.HasSuffix(stem, "."+ext) {
		return stem
	}
	return stem + "." + ext
}
