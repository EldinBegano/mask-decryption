# mask-decryption — Spec v0.1

## Purpose
CLI tool (GUI fast-follow) to encrypt/decrypt local files with modern authenticated encryption.
Encrypt: `file.txt` → `file.mlp`
Decrypt: `file.mlp` → `file.txt`

## Language / Stack
- Go (single static binary, cross-platform), min version: latest stable (1.23+)
- Module: `github.com/EldinBegano/mask-decryption`
- Binary/command name: `mlp`
- CLI framework: cobra
- GUI (v0.5, see ROADMAP.md): Fyne, as a separate binary from the CLI (needs CGO)
- License: GPL-3.0-or-later from v0.4 (repo goes public for the AUR release). Until then: none, all rights reserved.
- Output: plain text, no color, respects `NO_COLOR`; zero telemetry/analytics, ever.

## Crypto
- Algorithm: AES-256-GCM (AEAD — confidentiality + integrity in one)
- Key: 256-bit, generated via `crypto/rand`
- Nonce: 96-bit, **counter-based** (not random) — guarantees no reuse under a given key.
  - Counter stored in a separate state file next to the keyfile: `<config>/mask-decryption/counter`.
  - Incremented and fsynced *before* each encrypt uses the value (crash-safe: never reuse on interrupted write).
  - Nonce actually used = counter value, stored in file header alongside ciphertext.
  - If counter state file is missing/corrupted but keyfile is present: fall back to a random nonce for that operation and print a loud warning (does not block the operation).
- Tampered/corrupted `.mlp` file → GCM auth tag check fails → decrypt aborts with clear error, no partial output

## Key management
- Single symmetric keyfile, no passphrase.
- Location: OS config dir via `os.UserConfigDir()` → `<config>/mask-decryption/keyfile`
  (Linux: `~/.config/mask-decryption/keyfile`)
- Permissions: `0600` on creation.
- Rotation keeps the previous key as `<config>/mask-decryption/keyfile.old` (see `mlp rotate`).
- Auto-created on first `encrypt` if missing (no explicit `keygen` step required, but `mlp keygen` also exposed for manual regen — regen resets counter too).
- Program locates keyfile automatically — no path input from user in normal operation.
- **No recovery mechanism.** Lost/deleted keyfile = permanently unrecoverable data. On first key creation, CLI prints a one-time loud warning telling user to back up the keyfile.
- `mlp keyfile export <path>` — copies keyfile **and counter state** out (e.g. to USB) for backup, bundled together so a restore continues the counter correctly (avoids nonce reuse). Prints SHA-256 of the exported keyfile to terminal for manual verification.
- `mlp keyfile import <path>` — installs keyfile + counter from backup into the config dir (confirms before overwriting an existing one).
- Config dir override: `MLP_CONFIG_DIR` env var, if set, overrides `os.UserConfigDir()` default (portable/USB use, testing).

## File format (`.mlp`)
Binary header + ciphertext:

| Field | Size | Notes |
|---|---|---|
| Magic | 4 bytes | `"MLP1"` |
| Version | 1 byte | format version, `0x01` |
| Original extension | length-prefixed (1 byte len + UTF-8 bytes) | e.g. `"txt"`, restores extension on decrypt |
| Nonce | 12 bytes | GCM nonce, counter-derived (or random fallback if counter state was lost) |
| Ciphertext+Tag | remainder | AES-256-GCM output (tag appended) |

- Decrypt reads header, restores original extension automatically — user doesn't retype it.

## CLI
```
mlp encrypt <file|dir> [-o output]     # file.txt -> file.mlp (extension replaced, or custom path via -o/--output); dir = batch
mlp decrypt <file.mlp|dir> [-o output] # file.mlp -> file.txt (original extension restored from header, or custom path via -o/--output); dir = batch
mlp rotate <file.mlp|dir>... [-y]      # re-encrypt files under a new key, all-or-nothing (v0.2)
mlp verify <file.mlp>              # checks auth tag/integrity, no plaintext written to disk
mlp info <file.mlp>                # show header (format version, original extension, decrypts-to name, plaintext size); no key needed (v0.3)
mlp keygen                         # force-regenerate keyfile (with confirmation, old keyfile = old data unreadable)
mlp keyfile export <path>          # back up keyfile to given path
mlp keyfile import <path>          # restore keyfile from given path
```
- Built with cobra.
- `-o/--output` lets user redirect output location/filename; default is fixed naming next to input.
- Both original and output file are kept (no auto-delete).
- If output filename already exists: abort, don't overwrite, print error (no silent clobber).
  `--force` overwrite flag deferred to post-v0.1 (backlog).
- Default output: one-line confirmation printed on success (e.g. `encrypted -> file.mlp`). `-v/--verbose` adds step/timing detail. No progress bar (whole-file crypto is fast enough not to need one).
- Output file preserves original file's permission mode bits.
- Symlink input: followed (operates on link target), standard CLI behavior.

### Batch mode (v0.2): `mlp encrypt <dir>` / `mlp decrypt <dir>`
- Recursive, in place, one `.mlp` per file next to its source (mirrored tree, no bundled archive). Each file follows the single-file rules (keep both, never overwrite, mode bits preserved). `-o` with a directory is an error.
- Walk: hidden files/dirs included; symlinked files followed, symlinked directories not descended into (no loops, can't leave the tree); non-regular files (pipes, sockets, devices) skipped; the keystore config directory is always skipped.
- Encrypt skips files already ending in `.mlp` (counted in the summary, listed with `-v`). Decrypt ignores files that aren't `.mlp`.
- Same-name collision within one run (`notes.md`, `notes.txt` both -> `notes.mlp`): the later one falls back to appending, `notes.txt.mlp`; decrypt restores both names from the header. Only collisions with files created *in this run* fall back. A pre-existing output is an error, so re-running on an encrypted folder never creates duplicates.
- One file failing does not stop the run. Failures print as they happen, then `N encrypted, M skipped, K failed`. Exit code is the failures' shared code (e.g. 4), else 1.

### `mlp rotate <file.mlp|dir>...` (v0.2)
- Targets: `.mlp` files and/or directories (all `.mlp` inside). Symlinks resolved (the real file is replaced), duplicates collapsed. Prompts for confirmation unless `-y`.
- All-or-nothing: every target is decrypted with the current key and re-encrypted under an in-memory new key into a `.<name>.rotate-tmp` file beside it. Any failure (exit 3 on auth failure) deletes the temps and leaves the key and every file untouched.
- Only after all succeed: counter written (`max(old, used)`, never lowered), old key saved as `keyfile.old` (0600), new key activated, temps renamed over originals.
- `.mlp` files not included stay on the old key and no longer decrypt with the active one; `keyfile.old` keeps them recoverable. User deletes `keyfile.old` when done.
- A crash between key activation and the file renames is the one non-atomic window; recovery is via `keyfile.old` and any leftover `.rotate-tmp` files (a leftover blocks the next rotate until inspected).

### `mlp info <file.mlp>` (v0.3)
- Reads only the header: format version, original extension (`(none)` if empty), the filename `decrypt` would produce, and plaintext size (file size minus header minus the 16-byte GCM tag).
- Never touches the keystore, so it works with no keyfile, and creates nothing. It cannot detect tampering (ciphertext isn't authenticated without the key) — `mlp verify` does that.
- Exit 5 if the name doesn't end in `.mlp`; exit 1 for bad magic, unsupported version, truncated header, or a file too short to hold the auth tag.

## GUI (v0.5, see ROADMAP.md)
- Thin wrapper over same core library used by CLI (no duplicated crypto logic).
- File picker to choose input file.
- Buttons: Encrypt / Decrypt, calling same code path as CLI.
- Same "keep both files" and "no overwrite" behavior.
- Not part of v0.1 — CLI ships and stabilizes first.

## Scope (v0.1)
- CLI only. Single file only (directories/batch arrived in v0.2, see above).
- No password-based mode, no multi-key/multi-user support.

## Large files
- Whole-file AES-256-GCM: entire file loaded into memory, single seal/open call. Fine for typical personal files. No streaming/chunked AEAD in v0.1 (revisit if very-large-file use case shows up).

## Edge cases
- Encrypting a file already ending in `.mlp` → refused with error (no double-wrap).
- File with no extension → original-extension field in header stored as empty string; decrypt restores filename with no extension.
- Dotfiles (`.env`) have no extension: `.env` -> `.env.mlp`, restored exactly.

## Exit codes
Distinct codes per failure type, for scripting:
| Code | Meaning |
|---|---|
| 0 | success |
| 1 | generic error |
| 2 | keyfile missing/not found |
| 3 | auth/tamper failure (GCM tag mismatch on decrypt) |
| 4 | output file already exists |
| 5 | input already `.mlp` (encrypt) or not `.mlp` (decrypt) |
| 6 | `verify` failed (tamper/corruption detected) |

Batch runs (`encrypt`/`decrypt` on a directory) exit with the failures' shared code if they all match, otherwise 1.

## CI
- GitHub Actions: `go build` + `go vet` on every push/PR.
- goreleaser: cross-compiled binaries on tag push (separate workflow).

## Testing (v0.1)
- Minimal/none formally required for v0.1 — manual verification of encrypt/decrypt roundtrip, tamper detection, wrong/missing-keyfile behavior. Revisit adding unit + fuzz tests post-v0.1.

## Distribution
- goreleaser, cross-compiled binaries attached to GitHub releases on tag push.
- AUR (v0.4): `mlp` (source build) and `mlp-bin` (prebuilt release binary). See ROADMAP.md.

## Architecture
```
/cmd/mlp             - CLI entrypoint (cobra), builds the `mlp` binary
/cmd/gui             - Fyne entrypoint (v0.2, not yet scaffolded)
/internal/crypto     - AES-256-GCM encrypt/decrypt core
/internal/keystore   - keyfile + counter create/load/locate/export/import
/internal/fileformat - .mlp header read/write
```

## Backlog (post-v0.1, not open questions — deliberately deferred)
Scheduled into versions — see [ROADMAP.md](ROADMAP.md) for v0.2–v0.6 (batch
mode, key rotation, `mlp info`, AUR release, `--force`, GUI, docs).

Unscheduled:
- Streaming/chunked AEAD for very large files
- Automated test suite (unit + fuzz) — v0.7+
- Password-based mode — rejected permanently, not revisited

## Open questions
None — all resolved for v0.1.
