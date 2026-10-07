---
title: Troubleshooting
weight: 9
---

Error messages, what they mean, and what to do. Every error line starts
with `error:` on stderr; the exit code is in parentheses.

## `keyfile not found — run 'mlp encrypt' or 'mlp keygen' first` (2)

No keyfile in the [config directory](../concepts/how-it-works#where-the-key-lives).

- **New machine, or reinstalled?** Restore your backup:
  `mlp keyfile import <backup folder>`.
- **Using `MLP_CONFIG_DIR`?** Check it's set, and set to the folder that holds
  `keyfile` (not to the keyfile itself).
- **Never encrypted anything yet?** Then there's nothing to decrypt; `mlp
  encrypt` creates the key.

Don't run `mlp keygen` to "fix" this: a new key can't decrypt files made with
the old one.

## `authentication failed: data is corrupted, tampered with, or was encrypted with a different key` (3, or 6 from verify)

The file didn't pass GCM's integrity check, so nothing was written. In
practice, in rough order of likelihood:

1. **A different key is active.** After a `keygen`, `rotate` or
   `keyfile import`, files that weren't rotated are on the previous key. Check
   for `keyfile.old` in the config directory and
   [restore it](../backups#recovering-keyfileold-by-hand), or import the
   backup that matches the file.
2. **An older `mlp` is reading a newer file**, or v0.7.1 to v0.8.0 is reading
   a v0.6.0/v0.7.0 file. See the [compatibility table](../compatibility).
   Upgrade.
3. **The file is damaged**: an incomplete copy, a sync conflict, a failing
   disk. Get it from a backup if you have one.

`mlp info` still works on the file (it needs no key), which at least shows
whether the header is readable.

## `output file … already exists` (4)

`mlp` won't overwrite anything without being told. Either move the existing
file away, write elsewhere with `-o`, or replace it with `-f`.

On a folder, this is what you see when re-running an encrypt or decrypt that
was already done; it's harmless.

## `… is already a .mlp file` / `… is not a .mlp file` (5)

`encrypt` refuses `.mlp` input (no double encryption); `decrypt`, `verify`,
`info` and `rotate` need a name ending in `.mlp`. If you renamed an encrypted
file, rename it back to end in `.mlp`.

## `aborted, nothing was changed` (1)

A confirmation prompt was declined, or there was no input to answer it (a
script, cron, `</dev/null`). Pass `-y` to skip the prompt. See
[Prompts](../scripting#prompts).

## `fileformat: header sets a flag this build doesn't understand (file made by a newer mlp?)` (1)

The file uses a feature this `mlp` doesn't know, most likely brotli
compression read by v0.7.x. Upgrade.

## `unsupported .mlp format version` (1)

A version 2 file read by mlp v0.5 or older. Upgrade.

## `the keyfile was replaced while this was running …`

Something replaced the active key (a `keyfile import`, `keygen` or `rotate`,
possibly from `mlp-gui`) while a batch was running. The files already done
are fine, on the old key. Decide which key you want active, then run the
command again for the rest.

## `WARNING: nonce counter state was missing or corrupt — using random nonces instead.`

The `counter` file next to the keyfile is gone or damaged. The encrypt still
worked, with a random nonce instead of a counted one. To get back to
counting, export and re-import the key:

```sh
mlp keyfile export /tmp/mlp-fix        # backup gets a fresh, safe counter
mlp keyfile import /tmp/mlp-fix -y
rm -r /tmp/mlp-fix
```

## `WARNING: … could not restore the original timestamp (file itself is fine)`

The decrypted file is complete; the filesystem refused to set its
modification or access time (some network shares and FAT drives do). Nothing
to fix unless the dates matter to you.

## A `.rotate-tmp` file blocks `mlp rotate`

```text
error: … (a leftover .rotate-tmp file from an earlier crash? inspect it, then delete it)
```

An earlier rotate was interrupted. Check which key the leftover is under:

```sh
cp .notes.mlp.rotate-tmp /tmp/check.mlp && mlp verify /tmp/check.mlp
```

- **Passes:** the rotate got as far as activating the new key, and the
  leftover is the up-to-date copy. Move it over `notes.mlp`.
- **Fails:** it's an unused copy under a key that was never activated.
  Delete it.

## Still stuck?

[Open an issue](https://github.com/EldinBegano/mask-decryption/issues) with
the command, the full output, `mlp --version`, and your OS. Never attach your
keyfile.
