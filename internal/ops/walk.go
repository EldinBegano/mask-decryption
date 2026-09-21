// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

// Collected is a regular file found by CollectFiles.
type Collected struct {
	Path string
	Info fs.FileInfo
}

// WalkOutput is the result of CollectFiles.
type WalkOutput struct {
	Files   []Collected
	Skipped []string
	Failed  []error
}

// CollectFiles lists the regular files under root, in lexical order.
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
func CollectFiles(root string) (WalkOutput, error) {
	var out WalkOutput

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return out, err
	}
	cfg := configDirAbs()

	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			out.Failed = append(out.Failed, walkErr)
			return nil
		}

		if d.IsDir() {
			if cfg != "" {
				if abs, err := filepath.Abs(path); err == nil && abs == cfg {
					out.Skipped = append(out.Skipped, path+" (keystore directory)")
					return fs.SkipDir
				}
			}
			return nil
		}

		info, err := d.Info()
		if d.Type()&fs.ModeSymlink != 0 {
			info, err = os.Stat(path)
			if err == nil && info.IsDir() {
				out.Skipped = append(out.Skipped, path+" (symlinked directory)")
				return nil
			}
		}
		if err != nil {
			out.Failed = append(out.Failed, err)
			return nil
		}
		if !info.Mode().IsRegular() {
			out.Skipped = append(out.Skipped, path+" (not a regular file)")
			return nil
		}
		out.Files = append(out.Files, Collected{Path: path, Info: info})
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
