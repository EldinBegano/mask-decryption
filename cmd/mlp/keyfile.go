package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

func newKeyfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "keyfile",
		Short:         "Back up or restore the keyfile",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(newKeyfileExportCmd(), newKeyfileImportCmd())
	return cmd
}

func newKeyfileExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "export <path>",
		Short:         "Copy the keyfile and counter state to <path> for backup",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyfileExport(args[0])
		},
	}
}

func runKeyfileExport(dest string) error {
	if err := keystore.Export(dest); err != nil {
		if errors.Is(err, keystore.ErrKeyfileMissing) {
			return withCode(2, err)
		}
		return withCode(1, err)
	}
	sum, err := keystore.KeyfileSHA256()
	if err != nil {
		return withCode(1, err)
	}
	fmt.Printf("exported keyfile -> %s\n", dest)
	fmt.Printf("sha256: %s\n", sum)
	return nil
}

func newKeyfileImportCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:           "import <path>",
		Short:         "Restore a keyfile and counter state backup from <path>",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyfileImport(args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runKeyfileImport(src string, yes bool) error {
	exists, err := keystore.Exists()
	if err != nil {
		return withCode(1, err)
	}
	if exists && !yes {
		ok := confirm("A keyfile already exists. Importing replaces it and its nonce counter. Continue?")
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}
	if err := keystore.Import(src); err != nil {
		return withCode(1, err)
	}
	fmt.Println("keyfile imported")
	return nil
}

func confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
