// SPDX-License-Identifier: GPL-3.0-or-later

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
		Long:          `Back up or restore the keyfile and its nonce counter. See the export and import subcommands.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(newKeyfileExportCmd(), newKeyfileImportCmd())
	return cmd
}

func newKeyfileExportCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "export <path>",
		Short: "Copy the keyfile and counter state to <path> for backup",
		Long: `Copy the keyfile and its nonce counter into <path> (created if needed),
bundled together so a later import continues the counter correctly instead
of risking nonce reuse. Prints the SHA-256 of the copy as read back from
<path>, so you can check it later (e.g. after replugging a USB stick).

If <path> already holds a backup of a different key, asks before replacing
it (that may be the other key's only backup); -y replaces it without asking.
A backup of the same key is simply refreshed.

This is the only way to protect against keyfile loss: there is no other
recovery mechanism. Run it right after the first encrypt creates a
keyfile, and again after any 'mlp keygen' or 'mlp rotate'.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyfileExport(args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "replace a backup of a different key without asking")
	return cmd
}

func runKeyfileExport(dest string, yes bool) error {
	sum, counterReset, err := keystore.Export(dest, yes)
	if errors.Is(err, keystore.ErrBackupExists) {
		ok := confirm(fmt.Sprintf("%s already holds a backup of a different key. Replacing it loses that backup (copy it somewhere else first if you might still need it). Continue?", dest))
		if !ok {
			return withCode(1, errAborted)
		}
		sum, counterReset, err = keystore.Export(dest, true)
	}
	if err != nil {
		if errors.Is(err, keystore.ErrKeyfileMissing) {
			return withCode(2, err)
		}
		return withCode(1, err)
	}
	fmt.Printf("exported keyfile -> %s\n", dest)
	fmt.Printf("sha256: %s\n", sum)
	if counterReset {
		fmt.Fprintln(os.Stderr, "WARNING: nonce counter state was missing or corrupt, so the backup's counter starts at a")
		fmt.Fprintln(os.Stderr, "         random, very high value instead — that can't repeat a nonce the key already used.")
	}
	return nil
}

func newKeyfileImportCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Restore a keyfile and counter state backup from <path>",
		Long: `Install a keyfile+counter backup from <path> (as written by 'mlp keyfile
export') into the config directory, making it the active key.

The backup is checked first: a folder without a valid keyfile and counter
is refused before anything changes. If a different keyfile is active, it's
kept as keyfile.old (with its matching counter as counter.old) — restoring
the wrong backup by mistake is then recoverable by hand, not permanent. The
counter is never lowered: it becomes the higher of the backup's and the
current one.`,
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
		ok := confirm("A keyfile already exists. Importing replaces it — files encrypted with the current key can no longer be decrypted unless you restore it (it's kept as keyfile.old, with counter.old, if you pointed this at the wrong backup by mistake). Continue?")
		if !ok {
			return withCode(1, errAborted)
		}
	}

	replaced, err := keystore.Import(src)
	if err != nil {
		return withCode(1, err)
	}
	fmt.Println("keyfile imported")
	if replaced {
		oldPath, _ := keystore.OldKeyPath()
		fmt.Printf("previous key kept at %s — delete it once you no longer need it\n", oldPath)
	}
	return nil
}

// errAborted is returned when a confirmation prompt is declined, or can't be
// answered (no input). Nothing changed, but the command didn't do what was
// asked, so it must not exit 0: a script would read that as success.
var errAborted = errors.New("aborted, nothing was changed")

func confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
