package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

type collected struct {
	path string
	info fs.FileInfo
}

type walkOutput struct {
	files   []collected
	skipped []string
	failed  []error
}

// collectFiles lists the regular files under root, in lexical order.
//
//   - Hidden files and directories are included.
//   - Symlinked files are followed (same as single-file mode); symlinked
//     directories are not descended into, so a link can't cause a loop or
//     lead outside the tree.
//   - Non-regular files (sockets, devices, pipes) are skipped.
//   - The keystore's own config directory is skipped, so encrypting a big
//     tree like $HOME never touches the keyfile or counter.
//
// root itself may be a symlink; an explicitly named directory is followed.
func collectFiles(root string) (walkOutput, error) {
	var out walkOutput

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return out, err
	}
	cfg := configDirAbs()

	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			out.failed = append(out.failed, walkErr)
			return nil
		}

		if d.IsDir() {
			if cfg != "" {
				if abs, err := filepath.Abs(path); err == nil && abs == cfg {
					out.skipped = append(out.skipped, path+" (keystore directory)")
					return fs.SkipDir
				}
			}
			return nil
		}

		info, err := d.Info()
		if d.Type()&fs.ModeSymlink != 0 {
			info, err = os.Stat(path)
			if err == nil && info.IsDir() {
				out.skipped = append(out.skipped, path+" (symlinked directory)")
				return nil
			}
		}
		if err != nil {
			out.failed = append(out.failed, err)
			return nil
		}
		if !info.Mode().IsRegular() {
			out.skipped = append(out.skipped, path+" (not a regular file)")
			return nil
		}
		out.files = append(out.files, collected{path: path, info: info})
		return nil
	})
	return out, err
}

func configDirAbs() string {
	dir, err := keystore.ConfigDir()
	if err != nil {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return abs
}

// batchResult tallies a directory run. Failures are printed as they happen;
// the summary and exit code come from finish.
type batchResult struct {
	done    int
	skipped []string
	failed  []error
}

func (r *batchResult) addWalk(w walkOutput) {
	r.skipped = append(r.skipped, w.skipped...)
	for _, err := range w.failed {
		r.fail(err)
	}
}

func (r *batchResult) fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	r.failed = append(r.failed, err)
}

// finish prints the summary. With failures, the exit code is their shared
// code if they all agree, otherwise 1.
func (r *batchResult) finish(verb string) error {
	fmt.Printf("%d %s, %d skipped, %d failed\n", r.done, verb, len(r.skipped), len(r.failed))
	if verbose {
		for _, s := range r.skipped {
			fmt.Println("  skipped:", s)
		}
	}
	if len(r.failed) == 0 {
		return nil
	}

	code := 1
	for i, err := range r.failed {
		c := 1
		var ec *exitCodeErr
		if errors.As(err, &ec) {
			c = ec.code
		}
		if i == 0 {
			code = c
		} else if c != code {
			code = 1
			break
		}
	}
	return withCode(code, fmt.Errorf("%d file(s) failed", len(r.failed)))
}
