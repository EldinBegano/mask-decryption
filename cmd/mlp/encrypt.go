// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/fileformat"
	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

var warnedNonceFallback bool

func newEncryptCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:           "encrypt <file|dir>",
		Short:         "Encrypt a file (or every file in a directory, recursively) into .mlp files",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEncrypt(args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path for a single file (default: extension replaced with .mlp)")
	return cmd
}

// fileExt returns the extension without its dot. A name that is only a
// leading dot plus text (.env) is a dotfile with no extension.
func fileExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == base {
		return ""
	}
	return strings.TrimPrefix(ext, ".")
}

// defaultEncryptPath replaces the file's extension with .mlp: notes.txt -> notes.mlp.
// Extensionless names get it appended: README -> README.mlp, .env -> .env.mlp.
func defaultEncryptPath(inputPath string) string {
	if fileExt(inputPath) == "" {
		return inputPath + ".mlp"
	}
	return strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".mlp"
}

func warnNewKey(dir string) {
	fmt.Fprintf(os.Stderr, "WARNING: new keyfile created at %s\n", filepath.Join(dir, "keyfile"))
	fmt.Fprintln(os.Stderr, "         Back it up now: mlp keyfile export <path>")
	fmt.Fprintln(os.Stderr, "         If it's lost, every file encrypted with it becomes permanently unreadable.")
}

func checkOutputFree(outputPath string) error {
	if _, err := os.Stat(outputPath); err == nil {
		return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
	}
	return nil
}

func runEncrypt(inputPath, outputPath string) error {
	if info, err := os.Stat(inputPath); err == nil && info.IsDir() {
		if outputPath != "" {
			return withCode(1, errors.New("-o/--output cannot be used with a directory"))
		}
		return runEncryptDir(inputPath)
	}

	if strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is already a .mlp file", inputPath))
	}

	// os.Stat follows symlinks, so a symlink input operates on its target.
	info, err := os.Stat(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	if outputPath == "" {
		outputPath = defaultEncryptPath(inputPath)
	}
	if err := checkOutputFree(outputPath); err != nil {
		return err
	}

	key, created, err := keystore.LoadOrCreateKey()
	if err != nil {
		return withCode(2, err)
	}
	if created {
		dir, _ := keystore.ConfigDir()
		warnNewKey(dir)
	}

	return encryptOne(key, inputPath, outputPath, info)
}

// encryptOne encrypts one file to outputPath, refusing to overwrite. On any
// failure after the output is created, the partial output is removed.
func encryptOne(key []byte, inputPath, outputPath string, info os.FileInfo) error {
	if err := checkOutputFree(outputPath); err != nil {
		return err
	}

	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	nonce, fellBack, err := keystore.NextNonce()
	if err != nil {
		return withCode(1, err)
	}
	if fellBack && !warnedNonceFallback {
		warnedNonceFallback = true
		fmt.Fprintln(os.Stderr, "WARNING: nonce counter state was missing or corrupt — using random nonces instead.")
	}

	ciphertext, err := crypto.Encrypt(key, nonce[:], plaintext)
	if err != nil {
		return withCode(1, err)
	}

	ext := fileExt(inputPath)

	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return withCode(4, fmt.Errorf("output file %s already exists", outputPath))
		}
		return withCode(1, err)
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(outputPath)
		}
	}()

	if err := fileformat.WriteHeader(out, fileformat.Header{Ext: ext, Nonce: nonce}); err != nil {
		return withCode(1, err)
	}
	if _, err := out.Write(ciphertext); err != nil {
		return withCode(1, err)
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return withCode(1, err)
	}
	ok = true

	fmt.Printf("encrypted -> %s\n", outputPath)
	if verbose {
		fmt.Printf("  input:  %d bytes\n", len(plaintext))
		fmt.Printf("  output: %d bytes\n", len(ciphertext))
	}
	return nil
}

// runEncryptDir encrypts every regular file under root, in place, each to
// its own .mlp next to it. Two files in one folder that want the same name
// (notes.txt and notes.md -> notes.mlp) fall back to appending: the second
// becomes notes.md.mlp. Only collisions with files created in this run get
// that treatment; a pre-existing output is still an error, so re-running
// on an already-encrypted folder never spawns duplicates.
func runEncryptDir(root string) error {
	walk, err := collectFiles(root)
	if err != nil {
		return withCode(1, err)
	}

	var res batchResult
	res.addWalk(walk)

	key, created, err := keystore.LoadOrCreateKey()
	if err != nil {
		return withCode(2, err)
	}
	if created {
		dir, _ := keystore.ConfigDir()
		warnNewKey(dir)
	}

	createdThisRun := map[string]bool{}
	for _, f := range walk.files {
		if strings.HasSuffix(f.path, ".mlp") {
			res.skipped = append(res.skipped, f.path+" (already .mlp)")
			continue
		}
		outPath := defaultEncryptPath(f.path)
		if createdThisRun[outPath] {
			outPath = f.path + ".mlp"
		}
		if err := encryptOne(key, f.path, outPath, f.info); err != nil {
			res.fail(err)
			continue
		}
		createdThisRun[outPath] = true
		res.done++
	}
	return res.finish("encrypted")
}
