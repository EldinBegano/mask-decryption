---
title: Compatibility
weight: 8
---

**The current release reads every `.mlp` file any release has written.** If
you only ever upgrade, there's nothing to do. This page matters when an
*older* `mlp` (another machine, a pinned package) has to read newer files.

## Which version reads what

Rows are the `mlp` doing the decrypting, columns the version that encrypted
the file. Tested with real binaries built from each release tag.

| Reader ↓ / file from → | v0.1–v0.5 | v0.6.0, v0.7.0 | v0.7.1–v0.7.3 | v0.8.0+ (zstd or raw) | v0.8.0+ (brotli) |
|---|---|---|---|---|---|
| **v0.1–v0.5** | ✅ | ❌ unsupported version | ❌ unsupported version | ❌ unsupported version | ❌ unsupported version |
| **v0.6.0** | ✅ | ✅ ⚠️ see below | ❌ auth failed | ❌ auth failed | ❌ auth failed |
| **v0.7.0** | ✅ | ✅ | ❌ auth failed | ❌ auth failed | ❌ unknown flag |
| **v0.7.1–v0.7.3** | ✅ | ❌ auth failed | ✅ | ✅ | ❌ unknown flag |
| **v0.8.0** | ✅ | ❌ auth failed | ✅ | ✅ | ✅ |
| **v0.8.1+** | ✅ | ✅ with a note | ✅ | ✅ | ✅ |

Every ❌ fails loudly and writes nothing. "auth failed" looks exactly like a
wrong key or a tampered file (exit 3), which is confusing but safe.

Whether a v0.8+ file uses brotli depends on its contents: `mlp` picks
whatever compresses best. Typical text, source and binaries end up brotli;
very repetitive data can end up zstd, and already-compressed files raw.
(`mlp info` doesn't show which.)

## The known gaps

**v0.6.0 reading files from v0.7.0.** v0.7.0 compressed files with zstd, and
v0.6.0 predates compression and doesn't check for unknown header flags. It
decrypts those files "successfully" but writes out the still-compressed
bytes. This can't be fixed in a binary that's already released; just don't
use v0.6.0 on files from later versions.

**v0.7.1 to v0.8.0 refusing v0.6.0/v0.7.0 files.** v0.7.1 started binding
each header to its ciphertext, and those releases wrongly treated older,
unbound version 2 files as tampered. Fixed in v0.8.1, which decrypts them
with a note:

```text
note: old.mlp was made by mlp v0.6.0 or v0.7.0, whose headers aren't tamper-protected; re-encrypt it to add that
```

The contents of such files were always authenticated; only the header fields
(extension, timestamps, compression flag) weren't. To upgrade them in place,
run [`mlp rotate`](../commands/rotate) on them, or decrypt and re-encrypt.

## Version 1 files

Files from v0.1 to v0.5 use header version 1. Every release reads them. Their
contents are authenticated, but the header (just the stored extension) isn't
bound to it, so editing it isn't detected. `mlp rotate` converts them to
version 2.

## Keyfile and backups

The keyfile and counter haven't changed format since v0.1: a 32-byte key and
an 8-byte big-endian counter. A backup made by any version imports into the
current one. Since v0.8.1, `keyfile import` checks the backup is valid before
replacing anything.
