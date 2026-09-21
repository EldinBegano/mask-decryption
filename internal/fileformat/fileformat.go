// SPDX-License-Identifier: GPL-3.0-or-later

// Package fileformat reads and writes the .mlp container header:
// magic | version | original-extension | nonce, followed by AES-256-GCM
// ciphertext (with its tag appended) as the rest of the stream.
package fileformat

import (
	"errors"
	"fmt"
	"io"
)

const (
	Version   byte = 0x01
	NonceSize      = 12
	MaxExtLen      = 255
)

var magic = [4]byte{'M', 'L', 'P', '1'}

var (
	ErrBadMagic   = errors.New("not a valid .mlp file")
	ErrBadVersion = errors.New("unsupported .mlp format version")
)

// Header is the fixed metadata stored at the start of every .mlp file.
type Header struct {
	Ext   string // original file extension, without the leading dot; "" if none
	Nonce [NonceSize]byte
}

// Size is the number of bytes h occupies at the start of a .mlp file.
func (h Header) Size() int {
	return len(magic) + 1 + 1 + len(h.Ext) + NonceSize
}

// WriteHeader writes h to w.
func WriteHeader(w io.Writer, h Header) error {
	if len(h.Ext) > MaxExtLen {
		return fmt.Errorf("fileformat: extension too long (%d bytes)", len(h.Ext))
	}
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{Version}); err != nil {
		return err
	}
	if _, err := w.Write([]byte{byte(len(h.Ext))}); err != nil {
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
	if version[0] != Version {
		return h, ErrBadVersion
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
	return h, nil
}
