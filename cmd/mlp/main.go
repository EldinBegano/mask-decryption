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

	"github.com/EldinBegano/mask-decryption/internal/ops"
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

// exitCode maps an error to the process exit code from SPEC.md.
func exitCode(err error) int {
	var ec *exitCodeErr
	if errors.As(err, &ec) {
		return ec.code
	}
	switch ops.KindOf(err) {
	case ops.KindNoKey:
		return 2
	case ops.KindAuth:
		return 3
	case ops.KindExists:
		return 4
	case ops.KindWrongType:
		return 5
	}
	return 1
}

// opsErr wraps an ops error with its exit code.
func opsErr(err error) error {
	if err == nil {
		return nil
	}
	return withCode(exitCode(err), err)
}

var verbose bool

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCode(err))
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
