// SPDX-License-Identifier: GPL-3.0-or-later

// Command mlp encrypts and decrypts files with AES-256-GCM, using a
// single auto-managed keyfile (see internal/keystore) — no passphrase,
// no path input required from the user in normal operation.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// exitCodeErr carries a specific process exit code alongside the error,
// per SPEC.md's exit code table.
type exitCodeErr struct {
	code int
	err  error
}

func (e *exitCodeErr) Error() string { return e.err.Error() }
func (e *exitCodeErr) Unwrap() error { return e.err }

func withCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitCodeErr{code: code, err: err}
}

var verbose bool

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		code := 1
		var ec *exitCodeErr
		if errors.As(err, &ec) {
			code = ec.code
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(code)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "mlp",
		Short:         "mlp encrypts and decrypts files with AES-256-GCM",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print extra detail")

	root.AddCommand(
		newEncryptCmd(),
		newDecryptCmd(),
		newVerifyCmd(),
		newInfoCmd(),
		newRotateCmd(),
		newKeygenCmd(),
		newKeyfileCmd(),
	)
	return root
}
