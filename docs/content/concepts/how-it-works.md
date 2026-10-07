---
title: Keyfile and encryption
weight: 1
---

## Where the key lives

`mlp` keeps its state in one directory, found without any input from you:

| OS | Default directory |
|---|---|
| Linux | `$XDG_CONFIG_HOME/mask-decryption`, usually `~/.config/mask-decryption` |
| macOS | `~/Library/Application Support/mask-decryption` |
| Windows | `%AppData%\mask-decryption` |

Set `MLP_CONFIG_DIR` to use a different directory instead (a USB stick, a
test setup). Inside it:

| File | Contents |
|---|---|
| `keyfile` | The active key: 32 random bytes from the OS's secure random source. Mode `0600`. |
| `counter` | The nonce counter for the active key (see [Nonces](#nonces)). |
| `lock` | Empty; held while a command changes the key or counter (see [Running several at once](#running-several-at-once)). |
| `keyfile.old` | The previous key, written whenever `rotate`, `keygen` or `keyfile import` replaces the active one. |
| `counter.old` | The previous key's counter, written by `keyfile import` alongside `keyfile.old`. |

The first `mlp encrypt` creates the directory and the keyfile. Commands that
only read (`decrypt`, `verify`) never create a key; with none present they
exit 2.

## Encryption

Each file is encrypted with **AES-256-GCM**, an authenticated cipher: it
keeps the contents secret and also detects any change to them. In order,
`mlp encrypt`:

1. Reads the whole file into memory.
2. [Compresses](../compression) it, if that makes it smaller.
3. Takes the next nonce from the counter.
4. Builds the [header](../file-format) (original extension, nonce,
   timestamps, compression codec).
5. Encrypts, passing the header bytes as GCM's *additional authenticated
   data*. That binds the header to the ciphertext: change one byte of either
   and decryption fails.
6. Writes header + ciphertext + 16-byte tag to the `.mlp` file.

`mlp decrypt` reverses it, and fails with exit 3 before writing anything if
the authentication tag doesn't match: wrong key, corruption or tampering all
look the same, by design.

## Nonces

GCM needs a nonce (a "number used once") per encryption, and reusing one under
the same key is catastrophic: it leaks the XOR of the two plaintexts and lets
an attacker forge messages. So `mlp` doesn't pick nonces at random; it
**counts**:

- The 12-byte nonce is the counter value in its last 8 bytes (big-endian),
  giving 2<sup>64</sup> encryptions per key.
- The counter is advanced and written to disk **before** the nonce is used,
  atomically (temp file, fsync, rename). A crash at any point can waste a
  value, never repeat one.
- The counter is never lowered when the key changes. `keygen`, `rotate` and
  `keyfile import` all keep it at or above its previous value, so restoring
  an older key by hand can't lead to a repeat either.
- `mlp keyfile export` copies the counter together with the key, so a
  restored backup picks up where it left off.

If the counter file is missing or corrupt while the keyfile is fine, `mlp`
falls back to a random nonce for that encrypt and warns:

```text
WARNING: nonce counter state was missing or corrupt — using random nonces instead.
```

The operation still succeeds. Random 96-bit nonces are safe for a modest
number of files; the counter just removes the doubt entirely. Exporting a key
whose counter is lost gives the backup a counter starting at a random point
above 2<sup>63</sup>, far beyond anything the real count could have reached.

## Running several at once

Parallel scripts, or the CLI next to the GUI, are safe. Every change to the
keyfile or counter holds an exclusive lock on `<config>/lock` (`flock` on
Linux and macOS, `LockFileEx` on Windows; released automatically if the
process dies), so concurrent encrypts each get their own nonce, and two racing
first-ever encrypts create only one key.

A long batch keeps its key in memory. If the active keyfile is replaced
mid-run (say, a `keyfile import` in another terminal), the batch's remaining
files fail with:

```text
the keyfile was replaced while this was running (mlp keygen, rotate or keyfile import elsewhere?) — run it again
```

rather than encrypting under the old key with the new key's counter.

## Writing output

`mlp` never writes a partial or half-finished file under its real name:

1. The output goes to a temp file in the same directory (`.mlp-*.tmp`), which
   is fsynced.
2. It's then published:
   - normally by **hard-linking** it into place, which fails if the target
     exists, so nothing is clobbered, even by a file created a moment earlier
     by something else;
   - with `--force` by **renaming** it over the target.
3. The directory is fsynced (best effort).

An error or crash leaves the target either absent or complete. Filesystems
without hard links (FAT, exFAT, some network shares) fall back to "check, then
rename", which has a small race window.

Other rules that always hold:

- **The input is never deleted.** Both files exist afterwards; remove the
  original yourself.
- **Existing outputs aren't overwritten** without `-f`/`--force` (exit 4).
- **Input and output can't be the same file**, not even via a hard link, and
  the output can't be a directory. `--force` doesn't override either.
- **Symlinks are followed:** encrypting a link encrypts its target.

## What's preserved

| Kept | How |
|---|---|
| Original filename extension | Stored in the header; `notes.txt` → `notes.mlp` → `notes.txt`. |
| Permission bits | The output gets the source file's mode. |
| Modification and access time | Stored in the header, restored on decrypt. If restoring fails (odd filesystem), the file is kept and a warning printed. |

Not kept: owner and group (the output belongs to whoever runs `mlp`) and
ctime (which can't be set on Linux anyway).
