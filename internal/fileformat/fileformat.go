// SPDX-License-Identifier: GPL-3.0-or-later

// Package fileformat reads and writes the .mlp container header, followed by
// AES-256-GCM ciphertext (with its tag appended) as the rest of the stream.
// The ciphertext is the AEAD output of whatever plaintext was fed to it —
// this package has no idea whether that plaintext was compressed first; see
// Header.Compressed and package ops, which does the compressing.
//
// Version 1: magic | version | ext | nonce
// Version 2: magic | version | flags | ext | nonce | [mtime | atime]
//
// flagTimestamps in a version 2 header says whether the mtime/atime fields
// are present; a version 2 header written for a file whose original
// timestamps aren't known (e.g. mlp rotate on a version 1 source) omits
// them rather than inventing a value. Version 1 files are always read as
// having no timestamps. flagCompressed says the plaintext was zstd-compressed
// before encryption. Both flags live in the same byte introduced for
// timestamps, so adding compression didn't need another version bump.
//
// ReadHeader rejects a version 2 header with any flag bit it doesn't
// recognize (ErrUnknownFlags), rather than silently ignoring it: an older
// binary that doesn't understand a bit would otherwise decrypt successfully
// but hand back the wrong plaintext (e.g. still-compressed bytes) with no
// error. This guards future flags; it can't help a binary released before
// the check existed (mlp v0.6, which predates flagCompressed, has no such
// guard and will silently produce wrong output on a compressed file).
//
// Both versions decrypt; WriteHeader always writes the current version.
package fileformat

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	VersionNoTimestamps byte = 0x01 // no flags byte, no timestamps
	Version             byte = 0x02 // current: flags byte, optional timestamps

	flagTimestamps byte = 1 << 0
	flagCompressed byte = 1 << 1
	knownFlags     byte = flagTimestamps | flagCompressed

	NonceSize = 12
	MaxExtLen = 255

	timestampFieldSize = 12 // int64 unix seconds + uint32 nanoseconds
)

var magic = [4]byte{'M', 'L', 'P', '1'}

var (
	ErrBadMagic    = errors.New("not a valid .mlp file")
	ErrBadVersion  = errors.New("unsupported .mlp format version")
	ErrUnknownFlag = errors.New("fileformat: header sets a flag this build doesn't understand (file made by a newer mlp?)")
)

// Header is the fixed metadata stored at the start of every .mlp file.
type Header struct {
	Version byte   // set by ReadHeader; ignored by WriteHeader, which always writes Version
	Ext     string // original file extension, without the leading dot; "" if none
	Nonce   [NonceSize]byte

	// ModTime and AccessTime are the source file's timestamps at encryption
	// time. Zero if unknown: either read from a version 1 file, or from a
	// version 2 file whose header was written without them (see above).
	ModTime, AccessTime time.Time

	// Compressed reports whether the plaintext was zstd-compressed before
	// encryption; package ops decompresses after decrypting when set.
	Compressed bool
}

// HasTimestamps reports whether h carries a stored mtime/atime.
func (h Header) HasTimestamps() bool { return !h.ModTime.IsZero() }

// Size is the number of bytes h occupies at the start of a .mlp file, based
// on the version and fields ReadHeader populated it with.
func (h Header) Size() int {
	n := len(magic) + 1 + 1 + len(h.Ext) + NonceSize // magic, version, extLen, ext, nonce
	if h.Version == VersionNoTimestamps {
		return n
	}
	n++ // flags byte
	if h.HasTimestamps() {
		n += 2 * timestampFieldSize
	}
	return n
}

// WriteHeader writes h to w as the current version. Timestamps are included
// only if h.HasTimestamps().
func WriteHeader(w io.Writer, h Header) error {
	if len(h.Ext) > MaxExtLen {
		return fmt.Errorf("fileformat: extension too long (%d bytes)", len(h.Ext))
	}

	var flags byte
	if h.HasTimestamps() {
		flags |= flagTimestamps
	}
	if h.Compressed {
		flags |= flagCompressed
	}

	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{Version, flags, byte(len(h.Ext))}); err != nil {
		return err
	}
	if len(h.Ext) > 0 {
		if _, err := w.Write([]byte(h.Ext)); err != nil {
			return err
		}
	}
	if _, err := w.Write(h.Nonce[:]); err != nil {
		return err
	}
	if flags&flagTimestamps != 0 {
		if err := writeTimestamp(w, h.ModTime); err != nil {
			return err
		}
		if err := writeTimestamp(w, h.AccessTime); err != nil {
			return err
		}
	}
	return nil
}

// ReadHeader reads a Header from r, leaving r positioned at the start of
// the ciphertext.
func ReadHeader(r io.Reader) (Header, error) {
	var h Header

	var got [4]byte
	if _, err := io.ReadFull(r, got[:]); err != nil {
		return h, ErrBadMagic
	}
	if got != magic {
		return h, ErrBadMagic
	}

	var version [1]byte
	if _, err := io.ReadFull(r, version[:]); err != nil {
		return h, fmt.Errorf("fileformat: %w", err)
	}
	h.Version = version[0]
	if h.Version != VersionNoTimestamps && h.Version != Version {
		return h, ErrBadVersion
	}

	var flags byte
	if h.Version != VersionNoTimestamps {
		var flagByte [1]byte
		if _, err := io.ReadFull(r, flagByte[:]); err != nil {
			return h, fmt.Errorf("fileformat: %w", err)
		}
		flags = flagByte[0]
		if flags&^knownFlags != 0 {
			return h, ErrUnknownFlag
		}
		h.Compressed = flags&flagCompressed != 0
	}

	var extLen [1]byte
	if _, err := io.ReadFull(r, extLen[:]); err != nil {
		return h, fmt.Errorf("fileformat: %w", err)
	}
	if extLen[0] > 0 {
		buf := make([]byte, extLen[0])
		if _, err := io.ReadFull(r, buf); err != nil {
			return h, fmt.Errorf("fileformat: %w", err)
		}
		h.Ext = string(buf)
	}

	if _, err := io.ReadFull(r, h.Nonce[:]); err != nil {
		return h, fmt.Errorf("fileformat: %w", err)
	}

	if flags&flagTimestamps != 0 {
		mtime, err := readTimestamp(r)
		if err != nil {
			return h, err
		}
		atime, err := readTimestamp(r)
		if err != nil {
			return h, err
		}
		h.ModTime, h.AccessTime = mtime, atime
	}

	return h, nil
}

func writeTimestamp(w io.Writer, t time.Time) error {
	var buf [timestampFieldSize]byte
	binary.BigEndian.PutUint64(buf[0:8], uint64(t.Unix()))
	binary.BigEndian.PutUint32(buf[8:12], uint32(t.Nanosecond()))
	_, err := w.Write(buf[:])
	return err
}

func readTimestamp(r io.Reader) (time.Time, error) {
	var buf [timestampFieldSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return time.Time{}, fmt.Errorf("fileformat: %w", err)
	}
	sec := int64(binary.BigEndian.Uint64(buf[0:8]))
	nsec := int64(binary.BigEndian.Uint32(buf[8:12]))
	return time.Unix(sec, nsec).UTC(), nil
}
