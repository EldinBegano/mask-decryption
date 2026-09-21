// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/EldinBegano/mask-decryption/internal/ops"
)

// batchEvents prints each file as it finishes: successes to stdout, failures
// to stderr. Skips are only counted (listed with -v in finishBatch).
func batchEvents(verb string, showInSize bool) func(ops.Event) {
	return func(e ops.Event) {
		switch e.Kind {
		case ops.EventDone:
			reportDone(verb, e.Result, showInSize)
		case ops.EventFailed:
			fmt.Fprintln(os.Stderr, "error:", e.Err)
		}
	}
}

// finishBatch prints the summary. With failures, the exit code is their
// shared code if they all agree, otherwise 1.
func finishBatch(verb string, res ops.BatchResult) error {
	fmt.Printf("%d %s, %d skipped, %d failed\n", len(res.Done), verb, len(res.Skipped), len(res.Failed))
	if verbose {
		for _, s := range res.Skipped {
			fmt.Println("  skipped:", s)
		}
	}
	if len(res.Failed) == 0 {
		return nil
	}

	code := exitCode(res.Failed[0].Err)
	for _, f := range res.Failed[1:] {
		if exitCode(f.Err) != code {
			code = 1
			break
		}
	}
	return withCode(code, fmt.Errorf("%d file(s) failed", len(res.Failed)))
}
