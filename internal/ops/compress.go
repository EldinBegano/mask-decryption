// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/EldinBegano/mask-decryption/internal/fileformat"
)

// Brotli quality is chosen by input size so the worst case stays around a few
// seconds per file. Measured with this pure-Go implementation on one core:
//
//	q11  ~0.2-0.35 MB/s    q10  ~0.5-0.75 MB/s
//	q9   ~3-13 MB/s        q5   ~10-40 MB/s
//
// Even q5 compresses text, source and binaries better than zstd's best
// setting, so there's no size at which falling back to zstd for ratio pays.
const (
	tierMaxQ11 = 256 << 10 // <= 256 KiB: quality 11
	tierMaxQ10 = 2 << 20   // <= 2 MiB:   quality 10
	tierMaxQ9  = 64 << 20  // <= 64 MiB:  quality 9; bigger: quality 5

	// If the zstd pass leaves more than this fraction of the input, the
	// data is already compressed (jpg, zip, png, mp4, ...) and the slow brotli
	// passes would burn minutes (q11 runs at ~0.2 MB/s) to save a few percent.
	alreadyCompressedRatio = 0.95
)

func brotliQuality(n int) int {
	switch {
	case n <= tierMaxQ11:
		return 11
	case n <= tierMaxQ10:
		return 10
	case n <= tierMaxQ9:
		return 9
	default:
		return 5
	}
}

// The zstd encoder and decoder are safe for concurrent EncodeAll/DecodeAll
// calls and expensive to create, so one of each is shared by the whole
// process instead of one per file — batch runs were paying that setup cost
// once per file.
var (
	zstdEncOnce sync.Once
	zstdEnc     *zstd.Encoder
	zstdEncErr  error

	zstdDecOnce sync.Once
	zstdDec     *zstd.Decoder
	zstdDecErr  error
)

func zstdEncoder() (*zstd.Encoder, error) {
	zstdEncOnce.Do(func() {
		// No frame checksum: GCM already authenticates the plaintext, so the
		// 4 bytes per file would only duplicate that.
		zstdEnc, zstdEncErr = zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.SpeedDefault),
			zstd.WithEncoderCRC(false))
	})
	return zstdEnc, zstdEncErr
}

func zstdDecoder() (*zstd.Decoder, error) {
	zstdDecOnce.Do(func() { zstdDec, zstdDecErr = zstd.NewReader(nil) })
	return zstdDec, zstdDecErr
}

// compress returns the smallest of: data as-is, a zstd pass, and a
// size-tiered brotli pass. The brotli pass is skipped when the zstd pass
// shows the data is already compressed, so incompressible files cost almost
// nothing. The zstd result stays a candidate because it's already in hand and
// occasionally wins on tiny inputs.
func compress(data []byte) (out []byte, codec fileformat.Codec, err error) {
	if len(data) == 0 {
		return data, fileformat.CodecNone, nil
	}

	enc, err := zstdEncoder()
	if err != nil {
		return nil, 0, fmt.Errorf("ops: %w", err)
	}
	best, codec := data, fileformat.CodecNone
	fast := enc.EncodeAll(data, nil)
	if len(fast) < len(best) {
		best, codec = fast, fileformat.CodecZstd
	}
	if float64(len(fast)) > alreadyCompressedRatio*float64(len(data)) {
		return best, codec, nil
	}

	br, err := brotliCompress(data, brotliQuality(len(data)))
	if err != nil {
		return nil, 0, err
	}
	if len(br) < len(best) {
		best, codec = br, fileformat.CodecBrotli
	}
	return best, codec, nil
}

func brotliCompress(data []byte, quality int) ([]byte, error) {
	var buf bytes.Buffer
	w := brotli.NewWriterLevel(&buf, quality)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("ops: brotli: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("ops: brotli: %w", err)
	}
	return buf.Bytes(), nil
}

// decompress reverses compress for the given codec.
func decompress(data []byte, codec fileformat.Codec) ([]byte, error) {
	switch codec {
	case fileformat.CodecZstd:
		dec, err := zstdDecoder()
		if err != nil {
			return nil, fmt.Errorf("ops: %w", err)
		}
		out, err := dec.DecodeAll(data, nil)
		if err != nil {
			return nil, fmt.Errorf("ops: corrupt compressed data: %w", err)
		}
		return out, nil
	case fileformat.CodecBrotli:
		out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
		if err != nil {
			return nil, fmt.Errorf("ops: corrupt compressed data: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("ops: unknown codec %d", codec)
	}
}
