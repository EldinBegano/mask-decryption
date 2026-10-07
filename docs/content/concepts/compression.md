---
title: Compression
weight: 3
---

`mlp encrypt` compresses every file before encrypting it. There's no flag:
it always tries, and keeps whichever is smallest of

- the raw bytes,
- a zstd pass,
- a brotli pass.

The choice is recorded in the [header](../file-format#flags), and
`mlp decrypt` undoes it transparently.

Compression has to happen **before** encryption. Encrypted data looks random
and doesn't compress, so compressing a `.mlp` file afterwards (with zip, 7z,
…) gains essentially nothing.

## Already-compressed files

JPEGs, videos, zip archives and other `.mlp` files don't shrink further, and
brotli at high settings is slow on them. So the zstd pass runs first and
doubles as a detector: if it leaves more than 95% of the input, the file is
treated as already compressed, brotli is skipped, and the file is stored
raw (or as zstd, if that was still a little smaller). Such files cost well
under a second.

## Effort by file size

Brotli's strongest settings are slow, so the effort scales down as files get
bigger, keeping the worst case to a few seconds per file:

| File size | Brotli quality | Rough speed |
|---|---|---|
| up to 256 KiB | 11 | 0.2–0.35 MB/s |
| up to 2 MiB | 10 | 0.5–0.75 MB/s |
| up to 64 MiB | 9 | 3–13 MB/s |
| larger | 5 | 10–40 MB/s |

Speeds are for one core with the pure-Go implementation; your mileage will
vary. Even quality 5 beats zstd's best setting on text, source code and
binaries.

## What it buys

Measured against mlp v0.7.3, which only had zstd (real binaries, batch mode,
headers included):

| Data | Smaller by |
|---|---|
| Go source | 19.6% |
| JSON / HTML | 25.3% |
| Licenses, plain text | 26.0% |
| A 5.5 MB log file | 24.3% |
| Executables | 14.6% |
| Tiny files | 11.2% |
| Already compressed | 0.2% |

The cost is time on large numbers of small text files: a 15 MB folder of
JSON and HTML went from about 4 s to about 45 s.

## Limits

- Each file is compressed whole, in memory, on one core.
- Files are compressed independently. There's no 7z-style "solid" mode
  across files, which would conflict with every file being its own
  independent `.mlp`.
- `mlp info` doesn't say whether a file was compressed. Its `stored size`
  is the size of what was encrypted, which for a compressed file is the
  compressed size, not the original.
- Brotli files (written by v0.8.0 and later) can't be read by mlp v0.7.x,
  which refuses them with an "unknown flag" error. See
  [Compatibility](../../compatibility).
