# mlp

Encrypt and decrypt files with AES-256-GCM, using a single auto-managed
keyfile. No passphrase to remember, no key to type, no path to give it —
`mlp` finds its own keyfile automatically.

```
mlp encrypt notes.txt      notes.txt -> notes.mlp
mlp decrypt notes.mlp      notes.mlp -> notes.txt
```

**Documentation:** <https://eldinbegano.github.io/mask-decryption/> (source
in [`docs/`](docs/)). This README is the short version.

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [How it works, briefly](#how-it-works-briefly)
- [Command reference](#command-reference)
  - [`mlp encrypt`](#mlp-encrypt-filedir)
  - [`mlp decrypt`](#mlp-decrypt-filemlpdir)
  - [`mlp verify`](#mlp-verify-filemlp)
  - [`mlp info`](#mlp-info-filemlp)
  - [`mlp rotate`](#mlp-rotate-filemlpdir)
  - [`mlp keygen`](#mlp-keygen)
  - [`mlp keyfile export`](#mlp-keyfile-export-path)
  - [`mlp keyfile import`](#mlp-keyfile-import-path)
- [Exit codes](#exit-codes)
- [`mlp-gui`](#mlp-gui)
- [License](#license)

## Install

### Arch Linux (AUR)

```
yay -S mlp          # builds from source
# or
yay -S mlp-bin       # prebuilt binary, faster install
# or
yay -S mlp-gui       # desktop app (see below)
```

### From source

Needs Go 1.23 or newer.

```
git clone https://github.com/EldinBegano/mask-decryption
cd mask-decryption
go build -o mlp ./cmd/mlp
```

## Command reference

Every command also accepts `-v`/`--verbose` for extra detail, and
`-h`/`--help` for its own help text. Full detail lives in the man pages
(`man mlp`, `man mlp-encrypt`, etc., if installed) — this section is the
short version.

### `mlp encrypt <file|dir>`

```
mlp encrypt notes.txt              # notes.txt -> notes.mlp
mlp encrypt notes.txt -o out.mlp   # custom output path
mlp encrypt notes.txt -f           # replace an existing notes.mlp
mlp encrypt ~/Documents            # every file under it, recursively
```

Creates the keyfile on first use if none exists. Refuses to re-encrypt a
file that already ends in `.mlp`. Refuses to overwrite an existing output
file unless `-f`/`--force` is given.

### `mlp decrypt <file.mlp|dir>`

```
mlp decrypt notes.mlp              # notes.mlp -> notes.txt (name restored from header)
mlp decrypt notes.mlp -o out.txt   # custom output path
mlp decrypt notes.mlp -f           # replace an existing notes.txt
mlp decrypt ~/Documents            # every .mlp file under it, recursively
```

Fails loudly (exit 3) on a tampered or corrupted file — never writes a
partial or wrong-looking file.

Files made by mlp v0.6.0 or v0.7.0 decrypt normally, with a note: those
two releases didn't yet tie the header to the encrypted data, so their
headers aren't tamper-protected. Re-encrypting such a file (or `mlp
rotate`) adds that. (v0.7.1 to v0.8.0 wrongly refused these files as
tampered; upgrade to v0.8.1 or later to read them.)

### `mlp verify <file.mlp>`

Checks a file's authentication tag without writing any output. Needs the
keyfile (unlike `info`, below). Exits 6 on failure, 0 and prints `OK` on
success.

### `mlp info <file.mlp>`

Reads only the header — no key needed, nothing decrypted. Shows the
format version, original extension, the name it would decrypt to, its
stored size (the *compressed* size if the file was compressed — `info`
can't know the true original size without decrypting), and the stored
timestamps if any.

```
$ mlp info notes.mlp
file:            notes.mlp
format version:  2
original ext:    txt
decrypts to:     notes.txt
stored size:     148 bytes
modified:        2019-06-15T08:30:45+02:00
accessed:        2021-11-02T17:05:10+01:00
```

### `mlp rotate <file.mlp|dir>...`

Re-encrypts specific `.mlp` files under a freshly generated key, then
makes that key active — all-or-nothing: if any file fails, nothing
changes. This is the safe way to migrate specific files to a new key,
unlike `keygen` below, which just starts over.

```
mlp rotate secret.mlp
mlp rotate ~/Documents          # every .mlp file under it
mlp rotate secret.mlp -y        # skip the confirmation prompt
```

`.mlp` files you don't include stay on the old key and won't decrypt with
the new one active. The old key is kept as `keyfile.old` either way.

### `mlp keygen`

Generates a brand-new random key and makes it active. If a key already
exists, it's kept as `keyfile.old` first, so this is recoverable by hand
(restore `keyfile.old` back to `keyfile`) rather than permanent — but
nothing currently encrypted decrypts with the *new* key until you do that.
For migrating specific files to a new key without that manual step, use
`mlp rotate` instead.

```
mlp keygen        # asks for confirmation if a key already exists
mlp keygen -y      # skip the confirmation prompt
```

### `mlp keyfile export <path>`

```
mlp keyfile export /media/usb/backup
mlp keyfile export /media/usb/backup -y   # replace a different key's backup without asking
```

Copies the keyfile and its counter into `<path>` (created if needed),
bundled together so a later import continues the counter correctly. Prints
the SHA-256 of the copy as read back from `<path>`, so you can check it
again later. If `<path>` already holds a backup of a *different* key, it
asks before replacing it, since that may be the other key's only backup.
This is the backup step the warning at the top of this README is telling
you to run.

### `mlp keyfile import <path>`

```
mlp keyfile import /media/usb/backup
mlp keyfile import /media/usb/backup -y   # skip the confirmation prompt
```

Installs a keyfile+counter backup from `<path>`, making it active. The
backup is checked first: a folder without a valid keyfile and counter is
refused before anything changes. Keeps whatever different key was
previously active as `keyfile.old` (with its matching counter as
`counter.old`) first, so importing the wrong backup by mistake is
recoverable, not permanent.

## Exit codes

Scripts can rely on these:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | generic error |
| 2 | keyfile missing |
| 3 | authentication failed (tampered, corrupted, or wrong key) |
| 4 | output file already exists |
| 5 | wrong file type (already `.mlp` for encrypt, not `.mlp` for decrypt) |
| 6 | `verify` failed |

A batch run (`encrypt`/`decrypt` on a directory) exits with the shared
code of its failures if they all agree, otherwise 1. A declined
confirmation prompt — including one with no input to read, as in a
script — exits 1 and changes nothing; pass `-y` to skip the prompt.

Running several `mlp` commands at once (parallel scripts, the CLI next to
`mlp-gui`) is safe: they take turns on the keyfile and counter.

## `mlp-gui`

A desktop app for the same keyfile and `.mlp` format: pick or drop a file
or folder, encrypt or decrypt, back up or restore the keyfile from the
same window. Install it separately (`yay -S mlp-gui`) — it needs system
GL/X11 libraries the CLI doesn't, so it's a different package. Linux only
for now. The riskier commands (`rotate`, `keygen`, `verify`, `info`) are
CLI-only by design.

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).
