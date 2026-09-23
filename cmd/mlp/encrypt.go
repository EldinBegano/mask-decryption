// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

var warnedNonceFallback bool

func newEncryptCmd() *cobra.Command {
	var output string
	var force bool
	cmd := &cobra.Command{
		Use:   "encrypt <file|dir>",
		Short: "Encrypt a file (or every file in a directory, recursively) into .mlp files",
		Long: `Encrypt a file with AES-256-GCM, using the auto-managed keyfile (no
passphrase). notes.txt becomes notes.mlp; the original extension is stored
in the header and restored on decrypt.

Compression is tried automatically and kept only if it actually shrinks the
file. The source file's permission bits, modification time and access time
are also stored, and restored on decrypt.

Given a directory, every regular file under it (recursively, hidden files
included) is encrypted the same way, each to its own .mlp beside it. A file
already ending in .mlp is skipped, not re-encrypted. One file failing does
not stop the rest: after the run, a line like "N encrypted, M skipped, K
failed" summarizes it.

Refuses to overwrite an existing output file unless --force is given.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEncrypt(args[0], output, force)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path for a single file (default: extension replaced with .mlp)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "replace existing output files instead of failing")
	return cmd
}

func warnNewKey(dir string) {
	fmt.Fprintf(os.Stderr, "WARNING: new keyfile created at %s\n", filepath.Join(dir, "keyfile"))
	fmt.Fprintln(os.Stderr, "         Back it up now: mlp keyfile export <path>")
	fmt.Fprintln(os.Stderr, "         If it's lost, every file encrypted with it becomes permanently unreadable.")
}

// encryptKey loads the key, creating one (with a loud backup warning) on
// first use.
func encryptKey() ([]byte, error) {
	key, created, err := keystore.LoadOrCreateKey()
	if err != nil {
		return nil, err
	}
	if created {
		dir, _ := keystore.ConfigDir()
		warnNewKey(dir)
	}
	return key, nil
}

func warnNonceFallback() {
	if warnedNonceFallback {
		return
	}
	warnedNonceFallback = true
	fmt.Fprintln(os.Stderr, "WARNING: nonce counter state was missing or corrupt — using random nonces instead.")
}

func reportDone(verb string, r ops.Result, showInSize bool) {
	note := ""
	if r.Overwrote {
		note = " (overwrote existing)"
	}
	fmt.Printf("%s -> %s%s\n", verb, r.Output, note)
	if verbose {
		if showInSize {
			fmt.Printf("  input:  %d bytes\n", r.InSize)
		}
		fmt.Printf("  output: %d bytes\n", r.OutSize)
	}
	if r.NonceFallback {
		warnNonceFallback()
	}
	if r.TimestampFailed {
		fmt.Fprintf(os.Stderr, "WARNING: %s: could not restore the original timestamp (file itself is fine)\n", r.Output)
	}
}

func runEncrypt(inputPath, outputPath string, force bool) error {
	opts := ops.Options{Force: force}

	if info, err := os.Stat(inputPath); err == nil && info.IsDir() {
		if outputPath != "" {
			return withCode(1, errors.New("-o/--output cannot be used with a directory"))
		}
		res, err := ops.EncryptDir(encryptKey, inputPath, opts, batchEvents("encrypted", true))
		if err != nil {
			return opsErr(err)
		}
		return finishBatch("encrypted", res)
	}

	res, err := ops.EncryptFile(encryptKey, inputPath, outputPath, opts)
	if err != nil {
		return opsErr(err)
	}
	reportDone("encrypted", res, true)
	return nil
}
