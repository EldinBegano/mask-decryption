// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
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
		Use:           "decrypt <file.mlp|dir>",
		Short:         "Decrypt a .mlp file (or every .mlp file in a directory, recursively)",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDecrypt(args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path for a single file (default: original filename)")
	return cmd
}

func runDecrypt(inputPath, outputPath string) error {
	if info, err := os.Stat(inputPath); err == nil && info.IsDir() {
		if outputPath != "" {
			return withCode(1, errors.New("-o/--output cannot be used with a directory"))
		}
		return runDecryptDir(inputPath)
	}

	if !strings.HasSuffix(inputPath, ".mlp") {
		return withCode(5, fmt.Errorf("%s is not a .mlp file", inputPath))
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return withCode(1, err)
	}
	return decryptOne(keystore.LoadKey, inputPath, outputPath, info)
}

// readMLP parses the header and returns the ciphertext that follows it.
func readMLP(path string) (fileformat.Header, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileformat.Header{}, nil, err
	}
	defer f.Close()

	hdr, err := fileformat.ReadHeader(f)
	if err != nil {
		return hdr, nil, fmt.Errorf("%s: %w", path, err)
	}
	ciphertext, err := io.ReadAll(f)
	if err != nil {
		return hdr, nil, err
	}
	return hdr, ciphertext, nil
}

// decryptOne decrypts one .mlp file, refusing to overwrite. loadKey runs
// after the header parses, so a malformed file reports as malformed rather
// than as a missing keyfile. On failure after the output is created, the
// partial output is removed.
func decryptOne(loadKey func() ([]byte, error), inputPath, outputPath string, info os.FileInfo) error {
	hdr, ciphertext, err := readMLP(inputPath)
	if err != nil {
		return withCode(1, err)
	}

	key, err := loadKey()
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
	if err := checkOutputFree(outputPath); err != nil {
		return err
	}

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

	if _, err := out.Write(plaintext); err != nil {
		return withCode(1, err)
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return withCode(1, err)
	}
	ok = true

	fmt.Printf("decrypted -> %s\n", outputPath)
	if verbose {
		fmt.Printf("  output: %d bytes\n", len(plaintext))
	}
	return nil
}

// runDecryptDir decrypts every .mlp file under root, in place. Files that
// aren't .mlp are ignored.
func runDecryptDir(root string) error {
	walk, err := collectFiles(root)
	if err != nil {
		return withCode(1, err)
	}

	var res batchResult
	res.addWalk(walk)

	key, err := keystore.LoadKey()
	if err != nil {
		return withCode(2, err)
	}
	haveKey := func() ([]byte, error) { return key, nil }

	for _, f := range walk.files {
		if !strings.HasSuffix(f.path, ".mlp") {
			continue
		}
		if err := decryptOne(haveKey, f.path, "", f.info); err != nil {
			res.fail(err)
			continue
		}
		res.done++
	}
	return res.finish("decrypted")
}

// restoreExt appends "."+ext to stem unless stem already carries that
// extension. Handles the normal case (encrypt replaced the extension with
// .mlp), the fallback name (notes.md.mlp, stem already ends in .md), and
// files renamed via -o.
func restoreExt(stem, ext string) string {
	if ext == "" {
		return stem
	}
	if strings.HasSuffix(stem, "."+ext) {
		return stem
	}
	return stem + "." + ext
}
