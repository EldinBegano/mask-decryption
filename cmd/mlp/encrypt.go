package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/fileformat"
	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

func newEncryptCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:           "encrypt <file>",
		Short:         "Encrypt a file into a .mlp file",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEncrypt(args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path (default: <file>.mlp)")
	return cmd
}

func runEncrypt(inputPath, outputPath string) error {
	if strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is already a .mlp file", inputPath))
	}

	// os.Stat follows symlinks, so a symlink input operates on its target.
	info, err := os.Stat(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	if outputPath == "" {
		outputPath = strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".mlp"
	}
	if _, err := os.Stat(outputPath); err == nil {
		return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
	}

	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	key, created, err := keystore.LoadOrCreateKey()
	if err != nil {
		return withCode(2, err)
	}
	if created {
		dir, _ := keystore.ConfigDir()
		fmt.Fprintf(os.Stderr, "WARNING: new keyfile created at %s\n", filepath.Join(dir, "keyfile"))
		fmt.Fprintln(os.Stderr, "         Back it up now: mlp keyfile export <path>")
		fmt.Fprintln(os.Stderr, "         If it's lost, every file encrypted with it becomes permanently unreadable.")
	}

	nonce, fellBack, err := keystore.NextNonce()
	if err != nil {
		return withCode(1, err)
	}
	if fellBack {
		fmt.Fprintln(os.Stderr, "WARNING: nonce counter state was missing or corrupt — used a random nonce for this file instead.")
	}

	ciphertext, err := crypto.Encrypt(key, nonce[:], plaintext)
	if err != nil {
		return withCode(1, err)
	}

	ext := strings.TrimPrefix(filepath.Ext(inputPath), ".")

	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
		}
		return withCode(1, err)
	}
	defer out.Close()

	if err := fileformat.WriteHeader(out, fileformat.Header{Ext: ext, Nonce: nonce}); err != nil {
		return withCode(1, err)
	}
	if _, err := out.Write(ciphertext); err != nil {
		return withCode(1, err)
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return withCode(1, err)
	}

	fmt.Printf("encrypted -> %s\n", outputPath)
	if verbose {
		fmt.Printf("  input:  %d bytes\n", len(plaintext))
		fmt.Printf("  output: %d bytes\n", len(ciphertext))
	}
	return nil
}
