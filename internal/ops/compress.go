// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// maybeCompress zstd-compresses data and returns it only if that's actually
// smaller; otherwise it returns data unchanged. Already-compressed input
// (jpg, mp4, zip, another .mlp file) commonly doesn't shrink, so this is
// always tried and never assumed — same idea as 7z's own store-vs-deflate
// choice per file.
func maybeCompress(data []byte) (out []byte, compressed bool, err error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, false, fmt.Errorf("ops: %w", err)
	}
	defer enc.Close()

	packed := enc.EncodeAll(data, make([]byte, 0, len(data)))
	if len(packed) < len(data) {
		return packed, true, nil
	}
	return data, false, nil
}

// decompress reverses maybeCompress.
func decompress(data []byte) ([]byte, error) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("ops: %w", err)
	}
	defer dec.Close()

	out, err := dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("ops: corrupt compressed data: %w", err)
	}
	return out, nil
}
