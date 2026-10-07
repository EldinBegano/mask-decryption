---
title: The .mlp file format
linkTitle: File format
weight: 2
---

A `.mlp` file is a small binary header followed by the AES-256-GCM output.
Two header versions exist. `mlp` reads both and always writes the current
one, version 2.

`mlp info` shows what a header says without needing the key:

```console
$ mlp info notes.mlp
file:            notes.mlp
format version:  2
original ext:    txt
decrypts to:     notes.txt
stored size:     28 bytes
modified:        2026-10-07T12:10:08+02:00
accessed:        2026-10-07T12:10:08+02:00
```

## Version 2 (current, mlp v0.6+)

All multi-byte integers are big-endian.

| Field | Size | Notes |
|---|---|---|
| Magic | 4 bytes | ASCII `MLP1` |
| Version | 1 byte | `0x02` |
| Flags | 1 byte | See below. |
| Extension length | 1 byte | 0–255 |
| Extension | that many bytes | UTF-8, without the dot: `txt`. Empty for files with no extension. |
| Nonce | 12 bytes | GCM nonce: 4 zero bytes + the 8-byte counter value (or 12 random bytes after a [counter fallback](../how-it-works#nonces)). |
| Modification time | 12 bytes | Only if flag bit 0 is set. int64 Unix seconds + uint32 nanoseconds. |
| Access time | 12 bytes | Same, same condition. |
| Ciphertext + tag | rest of file | GCM output of the (possibly compressed) contents; the last 16 bytes are the authentication tag. |

### Flags

| Bit | Name | Meaning |
|---|---|---|
| 0 | timestamps | The two timestamp fields follow the nonce. |
| 1 | zstd | The contents were zstd-compressed before encryption (v0.7+). |
| 2 | brotli | The contents were brotli-compressed before encryption (v0.8+). |

Bits 1 and 2 are never both set; a header that sets both is rejected. Any
other bit is rejected too, rather than ignored: a file from a future `mlp`
that uses a new flag fails loudly instead of decrypting to subtly wrong
output.

Timestamps are optional because nothing is ever made up: rotating a version 1
file, which never stored them, produces a version 2 file without them, and
`mlp info` prints `timestamps:      (not stored)`.

### Tamper protection

The header's exact bytes are passed to GCM as additional authenticated data.
Change anything in it (extension, nonce, a timestamp, a compression flag)
and decryption fails with an authentication error, the same as if the
ciphertext itself had been altered. `mlp verify` checks this without writing
anything.

{{< callout type="info" >}}
Version 2 files made by **mlp v0.6.0 and v0.7.0** predate this binding: their
headers aren't protected, only their contents. They still decrypt, with a
note suggesting you re-encrypt them. See [Compatibility](../../compatibility).
{{< /callout >}}

## Version 1 (mlp v0.1–v0.5)

| Field | Size | Notes |
|---|---|---|
| Magic | 4 bytes | ASCII `MLP1` |
| Version | 1 byte | `0x01` |
| Extension length | 1 byte | |
| Extension | that many bytes | |
| Nonce | 12 bytes | |
| Ciphertext + tag | rest of file | Never compressed. |

No flags, no timestamps, and the header isn't bound to the ciphertext, so
editing a version 1 file's stored extension isn't detected. Its contents are
still fully authenticated. `mlp rotate` upgrades a version 1 file to version
2.

## Size overhead

A file grows by its header plus the 16-byte tag: 35 bytes for a version 2
file with no extension and no timestamps, plus the extension's length, plus
24 for timestamps. That's 62 bytes for a typical `notes.txt`, before
compression, which usually more than makes up for it: a 2,800-byte text file
can easily come out at 90 bytes.

## How the decrypted name is chosen

The `.mlp` suffix is replaced with the stored extension: `notes.mlp` with
extension `txt` decrypts to `notes.txt`; with no extension, to `notes`. When a
batch encrypt had to fall back to appending (`notes.txt.mlp`, because
`notes.mlp` was already taken by another file in the same run), the name
already ends in the extension and isn't doubled.
