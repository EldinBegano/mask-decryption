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
// before encryption, flagBrotli that it was brotli-compressed (at most one of
// the two). All of these flags live in the same byte introduced for
// timestamps, so adding compression codecs didn't need another version bump.
//
// ReadHeader rejects a version 2 header with any flag bit it doesn't
// recognize (ErrUnknownFlags), rather than silently ignoring it: an older
// binary that doesn't understand a bit would otherwise decrypt successfully
// but hand back the wrong plaintext (e.g. still-compressed bytes) with no
// error. This guards future flags; it can't help a binary released before
// the check existed (mlp v0.6, which predates flagCompressed, has no such
// guard and will silently produce wrong output on a compressed file).
//
// The header itself is never covered by the GCM tag on its own — only
// Header.AAD, bound in by the caller as GCM additional authenticated data,
// makes tampering with it (without the key) detectable. A version 2 header
// with no AAD binding would have the same silent-wrong-output problem as an
// unrecognized flag bit, just reachable by flipping a *recognized* one
// instead (see Header.AAD).
//
// Both versions decrypt; WriteHeader always writes the current version.
package fileformat

import (
	"bytes"
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
	flagCompressed byte = 1 << 1 // plaintext is zstd-compressed (v0.7)
	flagBrotli     byte = 1 << 2 // plaintext is brotli-compressed (v0.8)
	knownFlags     byte = flagTimestamps | flagCompressed | flagBrotli

	NonceSize = 12
	MaxExtLen = 255

	timestampFieldSize = 12 // int64 unix seconds + uint32 nanoseconds
)

var magic = [4]byte{'M', 'L', 'P', '1'}

var (
	ErrBadMagic    = errors.New("not a valid .mlp file")
	ErrBadVersion  = errors.New("unsupported .mlp format version")
	ErrUnknownFlag = errors.New("fileformat: header sets a flag this build doesn't understand (file made by a newer mlp?)")
	ErrBadFlags    = errors.New("fileformat: header sets conflicting compression flags")
)

// Codec says how a file's plaintext was compressed before encryption.
type Codec byte

const (
	CodecNone   Codec = iota // stored as-is
	CodecZstd                // zstd (mlp v0.7)
	CodecBrotli              // brotli (mlp v0.8+)
)

// Compressed reports whether c is anything other than CodecNone.
func (c Codec) Compressed() bool { return c != CodecNone }

// Header is the fixed metadata stored at the start of every .mlp file.
type Header struct {
	Version byte   // set by ReadHeader; ignored by WriteHeader, which always writes Version
	Ext     string // original file extension, without the leading dot; "" if none
	Nonce   [NonceSize]byte

	// ModTime and AccessTime are the source file's timestamps at encryption
	// time. Zero if unknown: either read from a version 1 file, or from a
	// version 2 file whose header was written without them (see above).
	ModTime, AccessTime time.Time

	// Codec is how the plaintext was compressed before encryption; package
	// ops decompresses after decrypting when it isn't CodecNone.
	Codec Codec
}

// HasTimestamps reports whether h carries a stored mtime/atime.
func (h Header) HasTimestamps() bool { return !h.ModTime.IsZero() }

// AAD returns the GCM additional authenticated data to use for h's
// ciphertext, given headerBytes — h's own encoded form, exactly as written
// to (EncodeHeader) or read from (ReadHeaderCapture) the .mlp file.
//
// For a version 1 header this is nil: those files predate AAD binding and
// were always encrypted with none, so decrypting one has to match that.
// For a version 2 header it's headerBytes itself, which is what stops the
// header — extension, timestamps, the compressed flag — from being changed
// independently of the ciphertext: any edit to those bytes, by anyone
// without the key, makes decryption fail instead of silently handing back
// data under the wrong header.
func (h Header) AAD(headerBytes []byte) []byte {
	if h.Version == VersionNoTimestamps {
		return nil
	}
	return headerBytes
}

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

// EncodeHeader returns the exact bytes WriteHeader would write for h, as the
// current version. Callers that need to bind a header to its ciphertext
// (see Header.AAD) must encode it this way *before* encrypting, then write
// those same bytes rather than calling WriteHeader separately — otherwise
// the bytes authenticated and the bytes on disk could drift apart.
func EncodeHeader(h Header) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeHeaderTo(&buf, h); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteHeader writes h to w as the current version. Timestamps are included
// only if h.HasTimestamps().
func WriteHeader(w io.Writer, h Header) error {
	return writeHeaderTo(w, h)
}

func writeHeaderTo(w io.Writer, h Header) error {
	if len(h.Ext) > MaxExtLen {
		return fmt.Errorf("fileformat: extension too long (%d bytes)", len(h.Ext))
	}

	var flags byte
	if h.HasTimestamps() {
		flags |= flagTimestamps
	}
	switch h.Codec {
	case CodecNone:
	case CodecZstd:
		flags |= flagCompressed
	case CodecBrotli:
		flags |= flagBrotli
	default:
		return fmt.Errorf("fileformat: unknown codec %d", h.Codec)
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
	h, _, err := ReadHeaderCapture(r)
	return h, err
}

// ReadHeaderCapture is like ReadHeader but also returns the exact bytes
// read for the header, which the caller must pass to Header.AAD when
// decrypting — this is what lets decrypt notice the header was tampered
// with independently of the ciphertext. headerBytes may be a truncated,
// partial capture when err != nil; callers only use it on success.
func ReadHeaderCapture(r io.Reader) (h Header, headerBytes []byte, err error) {
	var buf bytes.Buffer
	h, err = readHeaderFrom(io.TeeReader(r, &buf))
	return h, buf.Bytes(), err
}

func readHeaderFrom(r io.Reader) (Header, error) {
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
		switch {
		case flags&flagCompressed != 0 && flags&flagBrotli != 0:
			return h, ErrBadFlags
		case flags&flagCompressed != 0:
			h.Codec = CodecZstd
		case flags&flagBrotli != 0:
			h.Codec = CodecBrotli
		}
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
