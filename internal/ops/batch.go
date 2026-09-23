// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"strings"
)

// EventKind says what happened to one path during a batch.
type EventKind int

const (
	EventDone EventKind = iota
	EventSkipped
	EventFailed
)

// Event is reported through the callback as a batch progresses.
type Event struct {
	Kind   EventKind
	Result Result // set for EventDone
	Path   string // input path (EventSkipped: includes the reason)
	Output string // output attempted (EventFailed)
	Err    error  // set for EventFailed
}

// Failure is one file a batch could not process.
type Failure struct {
	Input, Output string
	Err           error
}

// BatchResult tallies a directory run.
type BatchResult struct {
	Done            []Result
	Skipped         []string
	Failed          []Failure
	NonceFallback   bool // some file used a random nonce (counter state lost)
	TimestampFailed bool // some decrypted file's stored timestamp could not be applied
}

// EncryptDir encrypts every regular file under root, in place, each to its
// own .mlp next to it. Two files in one folder that want the same name
// (notes.txt and notes.md -> notes.mlp) fall back to appending: the second
// becomes notes.md.mlp. Only collisions with files created in this run get
// that treatment; a pre-existing output is still an error (or replaced, with
// Force), so re-running on an already-encrypted folder never spawns
// duplicates.
//
// One failing file does not stop the run. A non-nil error means the run
// could not start (unreadable root, no key); on is called as files finish.
func EncryptDir(getKey KeyFunc, root string, opts Options, on func(Event)) (BatchResult, error) {
	var res BatchResult
	emit := emitter(&res, on)

	walk, err := CollectFiles(root)
	if err != nil {
		return res, wrap(KindOther, err)
	}
	for _, e := range walk.Failed {
		emit(Event{Kind: EventFailed, Err: e})
	}
	for _, s := range walk.Skipped {
		emit(Event{Kind: EventSkipped, Path: s})
	}

	key, err := getKey()
	if err != nil {
		return res, wrap(KindNoKey, err)
	}
	haveKey := func() ([]byte, error) { return key, nil }

	createdThisRun := map[string]bool{}
	for _, f := range walk.Files {
		if strings.HasSuffix(f.Path, ".mlp") {
			emit(Event{Kind: EventSkipped, Path: f.Path + " (already .mlp)"})
			continue
		}
		out := DefaultEncryptPath(f.Path)
		if createdThisRun[out] {
			out = f.Path + ".mlp"
		}
		r, err := EncryptFile(haveKey, f.Path, out, opts)
		if err != nil {
			emit(Event{Kind: EventFailed, Path: f.Path, Output: out, Err: err})
			continue
		}
		createdThisRun[out] = true
		emit(Event{Kind: EventDone, Result: r})
	}
	return res, nil
}

// DecryptDir decrypts every .mlp file under root, in place. Files that
// aren't .mlp are ignored.
func DecryptDir(getKey KeyFunc, root string, opts Options, on func(Event)) (BatchResult, error) {
	var res BatchResult
	emit := emitter(&res, on)

	walk, err := CollectFiles(root)
	if err != nil {
		return res, wrap(KindOther, err)
	}
	for _, e := range walk.Failed {
		emit(Event{Kind: EventFailed, Err: e})
	}
	for _, s := range walk.Skipped {
		emit(Event{Kind: EventSkipped, Path: s})
	}

	key, err := getKey()
	if err != nil {
		return res, wrap(KindNoKey, err)
	}
	haveKey := func() ([]byte, error) { return key, nil }

	for _, f := range walk.Files {
		if !strings.HasSuffix(f.Path, ".mlp") {
			continue
		}
		r, err := DecryptFile(haveKey, f.Path, "", opts)
		if err != nil {
			emit(Event{Kind: EventFailed, Path: f.Path, Output: r.Output, Err: err})
			continue
		}
		emit(Event{Kind: EventDone, Result: r})
	}
	return res, nil
}

// emitter records each event into res, then forwards it to on (if any).
func emitter(res *BatchResult, on func(Event)) func(Event) {
	return func(e Event) {
		switch e.Kind {
		case EventDone:
			res.Done = append(res.Done, e.Result)
			if e.Result.NonceFallback {
				res.NonceFallback = true
			}
			if e.Result.TimestampFailed {
				res.TimestampFailed = true
			}
		case EventSkipped:
			res.Skipped = append(res.Skipped, e.Path)
		case EventFailed:
			res.Failed = append(res.Failed, Failure{Input: e.Path, Output: e.Output, Err: e.Err})
		}
		if on != nil {
			on(e)
		}
	}
}
