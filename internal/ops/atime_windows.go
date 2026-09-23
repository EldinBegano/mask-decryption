// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package ops

import (
	"os"
	"syscall"
	"time"
)

// accessTime returns info's access time, falling back to its modification
// time if the platform doesn't expose one through Sys().
func accessTime(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, st.LastAccessTime.Nanoseconds())
	}
	return info.ModTime()
}
