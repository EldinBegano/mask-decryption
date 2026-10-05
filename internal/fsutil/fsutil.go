// SPDX-License-Identifier: GPL-3.0-or-later

// Package fsutil holds the file-system primitives mlp relies on for crash
// and concurrency safety: atomic file replacement, directory fsync, and an
// exclusive lock shared between processes.
package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// WriteFileAtomic replaces path with data: written to a temp file beside it,
// fsynced, renamed over path, then the directory fsynced so the rename
// itself survives a crash. path ends up holding either its old contents or
// the new ones, never a truncated mix.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()

	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(perm)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return SyncDir(filepath.Dir(path))
}

// SyncDir fsyncs a directory, making renames and new names in it durable.
// Filesystems that can't sync a directory report that as unsupported or
// invalid; that is not an error here. Windows has no equivalent call (NTFS
// journals renames itself), so it does nothing there.
func SyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil && !errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, syscall.EINVAL) {
		return err
	}
	return nil
}

// Lock takes an exclusive lock on the file at path (created if missing),
// blocking until no other process holds it. unlock releases it; so does the
// process exiting, so a crash never leaves it stuck.
func Lock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		unlockFile(f)
		f.Close()
	}, nil
}
