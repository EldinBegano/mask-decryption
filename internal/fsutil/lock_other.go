// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux && !darwin && !windows

package fsutil

import "os"

// No lock on this platform (not one mlp ships a build for): running two mlp
// processes at once here is unprotected, as it was before Lock existed.
func lockFile(*os.File) error { return nil }

func unlockFile(*os.File) error { return nil }
