// SPDX-License-Identifier: GPL-3.0-or-later

// Package ops implements the file-level encrypt and decrypt operations shared
// by the mlp CLI and the mlp-gui app. It never prints and never exits: callers
// decide how to report results and errors.
package ops

import "errors"

// Kind classifies an Error so callers can react (exit codes, dialogs)
// without matching on message text.
type Kind int

const (
	KindOther     Kind = iota
	KindNoKey          // keyfile missing or unreadable
	KindAuth           // authentication failed: wrong key, corruption or tampering
	KindExists         // output file already exists
	KindWrongType      // input is already .mlp (encrypt) or not .mlp (decrypt)
)

// Error is an operation failure with a Kind.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func wrap(kind Kind, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: kind, Err: err}
}

// KindOf returns the Kind of err, or KindOther if it has none.
func KindOf(err error) Kind {
	var oe *Error
	if errors.As(err, &oe) {
		return oe.Kind
	}
	return KindOther
}
