---
title: mlp rotate
linkTitle: rotate
weight: 5
---

```text
mlp rotate <file.mlp|dir>... [-y]
```

Re-encrypts the given `.mlp` files under a freshly generated key, then makes
that key the active one. Directories contribute every `.mlp` file under them.

| Flag | Meaning |
|---|---|
| `-y`, `--yes` | Skip the confirmation prompt. |

```console
$ mlp rotate notes.mlp taxes
Rotate 4 file(s) to a new key? .mlp files not included stay on the old key (kept as keyfile.old) and can no longer be decrypted with the active one. Continue? [y/N]: y
rotated notes.mlp
rotated taxes/2025.mlp
rotated taxes/2025.txt.mlp
rotated taxes/receipts.mlp
rotated 4 file(s) under a new key
old key kept at /home/you/.config/mask-decryption/keyfile.old — delete it once you no longer need it
WARNING: your keyfile changed. Back it up now: mlp keyfile export <path>
```

## All or nothing

1. Every file is decrypted with the current key and re-encrypted under the
   new one, into a temporary `.<name>.rotate-tmp` file beside it. The active
   key and the originals are untouched.
2. If **any** file fails (wrong key, corruption, a write error), all temp
   files are deleted and `mlp` exits with nothing changed. A file that fails
   to decrypt exits 3.
3. Only when every file succeeded: the old key is saved as `keyfile.old`, the
   new key becomes active, and each temp file is renamed over its original.

Rotation keeps each file's stored timestamps and compression as they are; it
moves the exact encrypted contents to the new key. Files from older formats
(version 1, or the unprotected headers of v0.6.0/v0.7.0) come out as current
version 2 files with a tamper-protected header.

Paths are resolved through symlinks (the real file is replaced, not the link)
and duplicates are collapsed, so naming a file twice is harmless.

## Files you leave out

`.mlp` files you **don't** pass to `rotate` stay encrypted under the old key,
which is no longer active, so they stop decrypting. `keyfile.old` keeps them
recoverable: see [Recovering keyfile.old](../../backups#recovering-keyfileold-by-hand).
Delete `keyfile.old` once nothing needs it.

Back up the new key afterwards. Your old backup is for the old key.

## If something goes wrong mid-way

The one non-atomic window is between activating the new key and renaming the
temp files. If a rename fails there, `mlp` says which files are still on the
old key and where their re-encrypted copies are:

```text
ERROR: the new key is active, but 1 file(s) could not be swapped in and are still on the OLD key.
Their re-encrypted copies (new key) were left next to them as:
  /home/you/taxes/.receipts.mlp.rotate-tmp  (replaces /home/you/taxes/receipts.mlp)
Move each into place by hand. The old key is at /home/you/.config/mask-decryption/keyfile.old
```

A leftover `.rotate-tmp` file (after a crash, say) blocks the next rotate of
that file until you've looked at it and deleted it.

If the counter file was missing or corrupt when the new key was committed,
`rotate` warns that a later restore of `keyfile.old` is safe for decrypting
but not for encrypting new files.

| Situation | Exit |
|---|---|
| All files rotated | 0 |
| Prompt declined, no `.mlp` files found, write error, or incomplete swap | 1 |
| No keyfile | 2 |
| A file failed to decrypt | 3 |
| A file argument doesn't end in `.mlp` | 5 |
