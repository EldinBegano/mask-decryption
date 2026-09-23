// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"os"
	"syscall"
	"time"
)

// accessTime returns info's access time, falling back to its modification
// time if the platform doesn't expose one through Sys().
func accessTime(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Atimespec.Sec, st.Atimespec.Nsec)
	}
	return info.ModTime()
}
