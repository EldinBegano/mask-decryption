// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux && !darwin && !windows

package ops

import (
	"os"
	"time"
)

// accessTime is not read on this platform (not one mlp ships a build for);
// falling back to ModTime keeps behavior sane if it's ever built here.
func accessTime(info os.FileInfo) time.Time {
	return info.ModTime()
}
