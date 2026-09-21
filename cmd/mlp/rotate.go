// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

func newRotateCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rotate <file.mlp|dir>...",
		Short: "Re-encrypt .mlp files under a freshly generated key",
		Long: `Re-encrypt the given .mlp files (or every .mlp file under the given
directories) under a new key, then make that key the active one.

All-or-nothing: every file is decrypted with the current key and re-encrypted
into a temporary file first. Only if all of them succeed is the new key
committed and the originals replaced. Any failure aborts with the current
key and every file untouched.

The old key is kept as keyfile.old. .mlp files you did not include stay on
the old key and can no longer be decrypted with the active one.`,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRotate(args, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

type rotateTarget struct {
	path string // resolved real path
	info os.FileInfo
}

type pendingRotate struct {
	tmp, dst string
}

func runRotate(args []string, yes bool) error {
	oldKey, err := keystore.LoadKey()
	if err != nil {
		return withCode(2, err)
	}

	targets, err := collectRotateTargets(args)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return withCode(1, fmt.Errorf("no .mlp files found in the given paths"))
	}

	if !yes {
		ok := confirm(fmt.Sprintf(
			"Rotate %d file(s) to a new key? .mlp files not included stay on the old key (kept as keyfile.old) and can no longer be decrypted with the active one. Continue?",
			len(targets)))
		if !ok {
			fmt.Println("aborted")
			return nil
		}
	}

	rot, err := keystore.BeginRotation()
	if err != nil {
		return withCode(1, err)
	}

	// Phase 1: re-encrypt everything into temp files next to the originals.
	// Nothing about the active key or any original changes yet.
	var pending []pendingRotate
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, p := range pending {
			os.Remove(p.tmp)
		}
	}()

	for _, t := range targets {
		hdr, ciphertext, err := ops.ReadMLP(t.path)
		if err != nil {
			return withCode(1, err)
		}
		plaintext, err := crypto.Decrypt(oldKey, hdr.Nonce[:], ciphertext)
		if err != nil {
			return withCode(3, fmt.Errorf("%s: %w (nothing was changed)", t.path, err))
		}

		nonce := rot.NextNonce()
		sealed, err := crypto.Encrypt(rot.Key(), nonce[:], plaintext)
		if err != nil {
			return withCode(1, err)
		}

		tmp := filepath.Join(filepath.Dir(t.path), "."+filepath.Base(t.path)+".rotate-tmp")
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return withCode(1, fmt.Errorf("%w (a leftover .rotate-tmp file from an earlier crash? inspect it, then delete it)", err))
		}
		pending = append(pending, pendingRotate{tmp: tmp, dst: t.path})

		werr := fileformat.WriteHeader(f, fileformat.Header{Ext: hdr.Ext, Nonce: nonce})
		if werr == nil {
			_, werr = f.Write(sealed)
		}
		if werr == nil {
			werr = f.Chmod(t.info.Mode().Perm())
		}
		if werr == nil {
			werr = f.Sync()
		}
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return withCode(1, fmt.Errorf("%s: %w", t.path, werr))
		}
	}

	// Phase 2: commit the new key, then swap the files in.
	if err := rot.Commit(); err != nil {
		return withCode(1, fmt.Errorf("could not activate the new key: %w (nothing was changed)", err))
	}
	committed = true

	var stuck []pendingRotate
	replaced := 0
	for _, p := range pending {
		if err := os.Rename(p.tmp, p.dst); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			stuck = append(stuck, p)
			continue
		}
		replaced++
		fmt.Printf("rotated %s\n", p.dst)
	}

	oldPath, _ := keystore.OldKeyPath()
	if len(stuck) > 0 {
		fmt.Fprintf(os.Stderr, "\nERROR: the new key is active, but %d file(s) could not be swapped in and are still on the OLD key.\n", len(stuck))
		fmt.Fprintf(os.Stderr, "Their re-encrypted copies (new key) were left next to them as:\n")
		for _, p := range stuck {
			fmt.Fprintf(os.Stderr, "  %s  (replaces %s)\n", p.tmp, p.dst)
		}
		fmt.Fprintf(os.Stderr, "Move each into place by hand. The old key is at %s\n", oldPath)
		return withCode(1, fmt.Errorf("rotation incomplete: %d of %d file(s) replaced", replaced, len(pending)))
	}

	fmt.Printf("rotated %d file(s) under a new key\n", replaced)
	fmt.Printf("old key kept at %s — delete it once you no longer need it\n", oldPath)
	fmt.Fprintln(os.Stderr, "WARNING: your keyfile changed. Back it up now: mlp keyfile export <path>")
	return nil
}

// collectRotateTargets expands the args into .mlp files: a file arg must end
// in .mlp; a directory arg contributes every .mlp under it. Paths are
// resolved through symlinks (rotating replaces the real file, not the
// link) and de-duplicated.
func collectRotateTargets(args []string) ([]rotateTarget, error) {
	seen := map[string]bool{}
	var targets []rotateTarget

	add := func(path string) error {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return withCode(1, err)
		}
		if seen[real] {
			return nil
		}
		info, err := os.Stat(real)
		if err != nil {
			return withCode(1, err)
		}
		seen[real] = true
		targets = append(targets, rotateTarget{path: real, info: info})
		return nil
	}

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, withCode(1, err)
		}

		if !info.IsDir() {
			if !strings.HasSuffix(arg, ".mlp") {
				return nil, withCode(5, fmt.Errorf("%s is not a .mlp file", arg))
			}
			if err := add(arg); err != nil {
				return nil, err
			}
			continue
		}

		walk, err := ops.CollectFiles(arg)
		if err != nil {
			return nil, withCode(1, err)
		}
		if len(walk.Failed) > 0 {
			return nil, withCode(1, fmt.Errorf("could not read everything under %s: %w", arg, walk.Failed[0]))
		}
		for _, f := range walk.Files {
			if !strings.HasSuffix(f.Path, ".mlp") {
				continue
			}
			if err := add(f.Path); err != nil {
				return nil, err
			}
		}
	}
	return targets, nil
}
