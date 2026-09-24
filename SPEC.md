# mask-decryption — Spec

## Purpose
CLI tool (GUI fast-follow) to encrypt/decrypt local files with modern authenticated encryption.
Encrypt: `file.txt` → `file.mlp`
Decrypt: `file.mlp` → `file.txt`

## Language / Stack
- Go (single static binary, cross-platform), min version: latest stable (1.23+)
- Module: `github.com/EldinBegano/mask-decryption`
- Binary/command name: `mlp`; `mlp --version` prints `mlp <version>` (`dev` for an untagged build)
- CLI framework: cobra
- GUI (v0.5, see ROADMAP.md): Fyne, as a separate binary from the CLI (needs CGO)
- License: GPL-3.0-or-later (`LICENSE`, SPDX header in every source file).
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
- Auto-created on first `encrypt` if missing (no explicit `keygen` step required, but `mlp keygen` also exposed for manual regen).
- Program locates keyfile automatically — no path input from user in normal operation.
- **No recovery mechanism for the *active* key.** Lost/deleted keyfile (and its `keyfile.old`, if any) = permanently unrecoverable data. On first key creation, CLI prints a one-time loud warning telling user to back up the keyfile.
- **Every operation that replaces the active key preserves the previous one as `keyfile.old`** — `mlp rotate` (the original case), `mlp keygen`, and `mlp keyfile import` all go through this same safety net, so one mistake (a `keygen` run by accident, or `keyfile import` pointed at the wrong path) is recoverable by hand rather than permanent. Single slot: each replacement overwrites whatever was there, it's "the previous key," not a history. Nothing is backed up when there was no existing key to replace (a genuinely first-ever `keygen`).
  - `mlp keygen`: backs up the key only (`keyfile.old`). The counter is **not** reset to 0 — like `mlp rotate`, it's a never-lowered high-water-mark across every key this config dir has ever had, so if `keyfile.old` is later restored by hand, resuming under it can't reuse a nonce it already used. A first-ever `keygen` (nothing to preserve) still starts the counter at 0.
  - `mlp keyfile import`: backs up the key **and its matched counter** together, as `keyfile.old` + `counter.old` — restoring that pair (not just the key alone) is what keeps resuming the old key nonce-safe, since `import` fully replaces the counter too (unlike `keygen`).
  - Both print where the backup landed (`previous key kept at <path> — delete it once you no longer need it`), same as `mlp rotate` already does.
- `mlp keyfile export <path>` — copies keyfile **and counter state** out (e.g. to USB) for backup, bundled together so a restore continues the counter correctly (avoids nonce reuse). Prints SHA-256 of the exported keyfile to terminal for manual verification.
- `mlp keyfile import <path>` — installs keyfile + counter from backup into the config dir (confirms before overwriting an existing one; see the `keyfile.old`/`counter.old` backup above).
- Config dir override: `MLP_CONFIG_DIR` env var, if set, overrides `os.UserConfigDir()` default (portable/USB use, testing).

## File format (`.mlp`)
Binary header + ciphertext. Two versions exist; `ReadHeader` accepts both, `WriteHeader` always writes the current one (`0x02`).

**Version `0x01`** (files from v0.1–v0.5):

| Field | Size | Notes |
|---|---|---|
| Magic | 4 bytes | `"MLP1"` |
| Version | 1 byte | `0x01` |
| Original extension | length-prefixed (1 byte len + UTF-8 bytes) | e.g. `"txt"`, restores extension on decrypt |
| Nonce | 12 bytes | GCM nonce, counter-derived (or random fallback if counter state was lost) |
| Ciphertext+Tag | remainder | AES-256-GCM output (tag appended) |

**Version `0x02`** (current, v0.6+): adds a flags byte. Bit 0 (`flagTimestamps`, v0.6) says the mtime/atime fields follow; bit 1 (`flagCompressed`, v0.7) says the plaintext was zstd-compressed before encryption; bit 2 (`flagBrotli`, v0.8) says it was brotli-compressed instead (never both). All share this one version — adding compression didn't need another bump, since the byte layout it needs (a flags byte that can gain more bits) already existed.

| Field | Size | Notes |
|---|---|---|
| Magic | 4 bytes | `"MLP1"` |
| Version | 1 byte | `0x02` |
| Flags | 1 byte | bit 0 = timestamps follow after the nonce; bit 1 = plaintext is zstd-compressed; bit 2 = plaintext is brotli-compressed (v0.8). Bits 1 and 2 together → `ErrBadFlags`; any other bit set → `ErrUnknownFlag`. Neither is silently ignored (see below) |
| Original extension | length-prefixed (1 byte len + UTF-8 bytes) | e.g. `"txt"`, restores extension on decrypt |
| Nonce | 12 bytes | GCM nonce, counter-derived (or random fallback if counter state was lost) |
| ModTime, AccessTime | 12 bytes each (int64 unix seconds + uint32 nanoseconds), only if the timestamps flag is set | omitted when re-encrypting a v1 file whose original timestamps were never known (e.g. `mlp rotate`) — never fabricated |
| Ciphertext+Tag | remainder | AES-256-GCM output of the (possibly compressed) plaintext, tag appended |

**Version `0x02` headers are cryptographically bound to their ciphertext**, using AES-GCM's additional authenticated data (AAD): the header's own encoded bytes are passed as AAD when encrypting, and the same bytes (as actually read from the file) are required to match on decrypt. Editing *anything* in a v2 header without the key — extension, nonce, timestamps, the compressed flag — makes decryption fail loudly (`ErrAuthFailed`) instead of silently succeeding under the tampered header. `internal/crypto.Encrypt`/`Decrypt` take this as an explicit `aad []byte` parameter; `fileformat.Header.AAD(headerBytes)` decides what to pass (nil for a version `0x01` header, `headerBytes` for version `0x02`).

**Version `0x01` headers are not bound** (extension only, since v1 predates timestamps/compression) — those files were released as far back as v0.1, always encrypted with no AAD, and stay exactly as they were; that gap can't be closed after the fact. Tampering with a v1 file's extension still isn't caught by `verify` or `decrypt`.

`ReadHeader` also rejects a version `0x02` header with any flag bit outside `flagTimestamps | flagCompressed | flagBrotli`, rather than ignoring it: an older binary that doesn't recognize a bit would otherwise decrypt successfully but hand back the wrong plaintext (e.g. still-compressed bytes written out raw) with no error at all. This guards future flag bits (it's what makes brotli files fail loudly, not silently, on v0.7.x — verified against the real released v0.7.3 binary: it refuses them with the unknown-flag error and writes nothing). It can't help **mlp v0.6 binaries already released** — they predate this check (and predate AAD binding entirely) and have no such guard, so a v0.6 binary opening a v0.7-compressed file will still silently produce corrupt (still-compressed) output. Not fixable after the fact; documented as a known gap of the v0.6 release.

- Decrypt reads header, restores original extension automatically — user doesn't retype it.

## CLI
```
mlp encrypt <file|dir> [-o output] [-f]     # file.txt -> file.mlp (extension replaced, or custom path via -o/--output); dir = batch
mlp decrypt <file.mlp|dir> [-o output] [-f] # file.mlp -> file.txt (original extension restored from header, or custom path via -o/--output); dir = batch
mlp rotate <file.mlp|dir>... [-y]      # re-encrypt files under a new key, all-or-nothing (v0.2)
mlp verify <file.mlp>              # checks auth tag/integrity, no plaintext written to disk
mlp info <file.mlp>                # show header (format version, original extension, decrypts-to name, stored size); no key needed (v0.3)
mlp keygen                         # force-regenerate keyfile (with confirmation; old key kept as keyfile.old, not lost)
mlp keyfile export <path>          # back up keyfile to given path
mlp keyfile import <path>          # restore keyfile from given path
```
- Built with cobra. Every command has a real `Long` description (not just `Short`), which also drives the generated man pages (v0.8, see Distribution) — one source of truth for both.
- `mlp gendoc <dir>` also exists, generating those man pages; it's hidden from `--help` since it's a build-time tool, not something to run day to day.
- `-o/--output` lets user redirect output location/filename; default is fixed naming next to input.
- Both original and output file are kept (no auto-delete).
- If output filename already exists: abort, don't overwrite, print error (no silent clobber). `-f/--force` replaces it instead (v0.5, below). Checked as early as possible — right after the output path is known (which for decrypt only needs the header's stored extension, not the key) and before any key fetch or decryption — so this cheap, local check reports first if it's the real blocker rather than being masked by a `getKey()` failure that would otherwise be hit first.
- Default output: one-line confirmation printed on success (e.g. `encrypted -> file.mlp`). `-v/--verbose` adds step/timing detail. No progress bar (whole-file crypto is fast enough not to need one).
- Output file preserves original file's permission mode bits.
- Encrypt stores the source file's mtime and atime in the header (v0.6+); decrypt restores them via `os.Chtimes` after writing. If that fails (odd filesystem, permission quirk), the decrypted file is kept and a warning is printed — metadata failing doesn't fail the operation. A `.mlp` with no stored timestamp (v1-format, or a v2 file rotated from one) decrypts with a fresh timestamp, same as before v0.6. Owner/group and ctime are never touched (see "what changes on encrypt" — size grows by the header+tag overhead, owner is never copied, ctime can't be set on Linux regardless).
- Symlink input: followed (operates on link target), standard CLI behavior.

### `--force` / `-f` (v0.5)
- On `encrypt` and `decrypt`, single file or directory: replace an existing output file instead of failing with exit 4. The result line says `(overwrote existing)`.
- Atomic: the new file is fully written to a temp file beside the target (`.<name>.*.tmp`), fsynced, and renamed over it. Any failure leaves the existing file untouched and no temp file behind. The replaced file takes the source file's permission bits.
- Guards that `--force` does **not** override: output being the same file as the input (including via a hard link) and output being a directory both exit 1.
- In batch mode it applies per file. The same-run collision fallback still applies (`notes.txt` still becomes `notes.txt.mlp`, never clobbering `notes.mlp` from the same run).
- Not offered on `rotate` (which replaces files by design, all-or-nothing) or `keygen`/`keyfile import` (which prompt or take `-y`).

### Batch mode (v0.2): `mlp encrypt <dir>` / `mlp decrypt <dir>`
- Recursive, in place, one `.mlp` per file next to its source (mirrored tree, no bundled archive). Each file follows the single-file rules (keep both, never overwrite, mode bits preserved). `-o` with a directory is an error.
- Walk: hidden files/dirs included; symlinked files followed, symlinked directories not descended into (no loops, can't leave the tree); non-regular files (pipes, sockets, devices) skipped; the keystore config directory is always skipped.
- Encrypt skips files already ending in `.mlp` (counted in the summary, listed with `-v`). Decrypt ignores files that aren't `.mlp`.
- Same-name collision within one run (`notes.md`, `notes.txt` both -> `notes.mlp`): the later one falls back to appending, `notes.txt.mlp`; decrypt restores both names from the header. Only collisions with files created *in this run* fall back. A pre-existing output is an error, so re-running on an encrypted folder never creates duplicates.
- One file failing does not stop the run. Failures print as they happen, then `N encrypted, M skipped, K failed`. Exit code is the failures' shared code (e.g. 4), else 1.

### `mlp rotate <file.mlp|dir>...` (v0.2)
- Targets: `.mlp` files and/or directories (all `.mlp` inside). Symlinks resolved (the real file is replaced), duplicates collapsed. Prompts for confirmation unless `-y`.
- All-or-nothing: every target is decrypted with the current key and re-encrypted under an in-memory new key into a `.<name>.rotate-tmp` file beside it. Any failure (exit 3 on auth failure) deletes the temps and leaves the key and every file untouched.
- Only after all succeed: counter written (`max(old, used)`, never lowered), old key saved as `keyfile.old` (0600), new key activated, temps renamed over originals. If the counter state was missing or corrupt at that point, it can't know the old key's true prior usage — it's treated as 0 for this write (so `max` doesn't protect it as normal), and a loud warning explains that a later manual restore of `keyfile.old` is safe to decrypt with but not to encrypt new files under (same fallback `mlp encrypt` already warns about via `NextNonce`, applied consistently here too).
- `.mlp` files not included stay on the old key and no longer decrypt with the active one; `keyfile.old` keeps them recoverable. User deletes `keyfile.old` when done.
- A crash between key activation and the file renames is the one non-atomic window; recovery is via `keyfile.old` and any leftover `.rotate-tmp` files (a leftover blocks the next rotate until inspected).

### `mlp info <file.mlp>` (v0.3, timestamps added v0.6)
- Reads only the header: format version (the file's actual version, `1` or `2` — not the tool's current write version), original extension (`(none)` if empty), the filename `decrypt` would produce, stored size (file size minus header minus the 16-byte GCM tag — the *compressed* size if the file used compression, v0.7+; `info` never decrypts, so it can't show the true original size for a compressed file), and, if stored, the mtime/atime in local time (RFC 3339). A file with no stored timestamps (v1, or rotated from one) prints `timestamps:      (not stored)` instead.
- Never touches the keystore, so it works with no keyfile, and creates nothing. It cannot detect tampering (ciphertext isn't authenticated without the key) — `mlp verify` does that.
- Exit 5 if the name doesn't end in `.mlp`; exit 1 for bad magic, unsupported version, truncated header, or a file too short to hold the auth tag.

## GUI: `mlp-gui` (v0.5)
A separate desktop binary, built from `cmd/mlp-gui` with Fyne. It is a thin front end over the same `internal/ops` and `internal/keystore` packages the CLI uses, so it shares the CLI's keyfile, `.mlp` format, and behavior (keep both files, never overwrite silently, permission bits preserved, batch rules unchanged).

- **Pick or drop:** one file or one folder, via "Choose file…", "Choose folder…", or drag and drop (first item if several are dropped). Buttons enable by what's selected: a plain file allows Encrypt only, a `.mlp` file Decrypt only, a folder both.
- **Run:** works on a background goroutine with a busy indicator; per-file results appear live in a list, then a summary (`N encrypted, M skipped, K failed`).
- **Overwrite prompt (the GUI's `--force`):** if outputs already exist, a dialog lists them and asks to replace. Yes redoes exactly those files with `Force`; No leaves them.
- **First-run notice:** when encrypting creates the keyfile, a dialog says where it is and to back it up.
- **Keyfile dialog:** shows the keyfile path; "Export backup…" (writes keyfile + counter to a chosen folder, shows the SHA-256) and "Import backup…" (confirms before replacing an existing key).
- **Decrypt with no keyfile:** explains and points to Import backup.
- Deliberately CLI-only: `rotate`, `keygen`, `verify`, `info`.
- `mlp-gui --version` prints `mlp-gui <version>` without opening a window.
- Needs CGO and system GL/X11/Wayland libraries, so it is not part of the CLI build, the goreleaser archives, or `mlp`/`mlp-bin`. Linux only for now (AUR `mlp-gui`); no macOS/Windows builds.

## Compression (v0.7, brotli added v0.8)
- Automatic, no flag: `encrypt` compresses the plaintext before encrypting (compressing ciphertext would gain nothing) and keeps whichever is smallest of the raw bytes, a zstd pass, and a brotli pass. The winner is recorded in the header (`flagCompressed` = zstd, `flagBrotli` = brotli, neither = stored raw); `decrypt` reads it and decompresses after the AEAD step, transparently. `mlp info` doesn't report whether or how much a file was compressed; its size field is labelled `stored size:` because for a compressed file that is what it is (see File format).
- **Already-compressed data is detected cheaply.** A zstd pass (default level, shared encoder, no frame checksum since GCM already authenticates the plaintext) always runs first. If it leaves more than 95% of the input, the file is treated as already compressed (jpg, mp4, zip, png, another `.mlp`), the brotli pass is skipped, and the zstd result is used only if smaller than raw. Without this, quality-11 brotli spent ~9 s/MB on such files.
- **Brotli effort is tiered by input size** so the worst case stays around a few seconds per file (pure Go, one core, measured): <= 256 KiB quality 11 (~0.2-0.35 MB/s), <= 2 MiB quality 10 (~0.5-0.75 MB/s), <= 64 MiB quality 9 (~3-13 MB/s), larger quality 5 (~10-40 MB/s). Quality 5 already beats zstd's best setting on text, source and binaries, so there is no size at which falling back to zstd for ratio pays. Larger brotli windows gave no gain at these sizes and aren't used.
- **Measured against v0.7.3** (real binaries, batch mode, per-file, headers included): Go source 19.6% smaller, JSON/HTML 25.3%, licenses/plain text 26.0%, a 5.5 MB log 24.3%, executables 14.6%, tiny files 11.2%, already-compressed data 0.2% smaller (and skipped in under a second). The price is time on many small text files: JSON/HTML 4 s -> ~45 s, source 1.4 s -> ~15 s, per 15 MB / 4.5 MB. Dropping quality 11 would give back ~2% of ratio for ~1.7x less time.
- Libraries: `klauspost/compress` v1.18.4 (zstd; newest patch needing only Go 1.23) and `andybalholm/brotli` v1.2.4 (needs Go 1.22). Both pure Go, no CGO. The zstd encoder/decoder are created once and shared, not per file (batch runs used to pay that setup per file).
- Compatibility: this release still reads every earlier file (format v1, and v2 with zstd). Files written by v0.8+ that chose brotli can't be read by v0.7.x, which refuses them loudly; v0.6.x predates compression entirely (see the known gap in File format).
- `mlp rotate` never decompresses/recompresses: it moves the exact decrypted bytes (compressed or not) to a new key and carries the header's codec through unchanged.
- No new command-line surface: this only touches what `encrypt`/`decrypt` do internally, including in batch mode.

## Scope (v0.1)
- CLI only. Single file only (directories/batch arrived in v0.2, see above).
- No password-based mode, no multi-key/multi-user support.

## Large files
- Whole-file AES-256-GCM: entire file loaded into memory, single seal/open call. Fine for typical personal files. No streaming/chunked AEAD in v0.1 (revisit if very-large-file use case shows up). Compression (v0.7) is also whole-file/in-memory, same constraint.

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
- goreleaser, cross-compiled binaries attached to GitHub releases on tag push. Every release archive includes `LICENSE`, shell completions (`completions/`), and man pages (`man/`, one per command, v0.8+).
- AUR: `mlp` (source build), `mlp-bin` (prebuilt release binary) and `mlp-gui` (source build of the GUI, v0.5), pushed automatically on each stable tag by `.github/workflows/release.yml` using `packaging/aur/`. Both `mlp` and `mlp-bin` install the man pages to `/usr/share/man/man1/`. See ROADMAP.md.
- `README.md` (v0.8): install, quick start, command reference, exit codes — the user-facing counterpart to this file.

## Architecture
```
/cmd/mlp             - CLI entrypoint (cobra), builds the `mlp` binary; thin wrapper over internal/ops
/cmd/mlp-gui         - Fyne desktop app, builds the `mlp-gui` binary (CGO); assets/ holds the icon and .desktop file
/internal/ops        - file-level encrypt/decrypt, batch walk, atomic writes; never prints or exits (shared by CLI and GUI)
                        atime_{linux,darwin,windows,other}.go - per-OS access-time extraction (os.FileInfo doesn't expose it portably)
                        compress.go - picks raw / zstd / size-tiered brotli, and decompresses (v0.7, brotli v0.8)
/internal/crypto     - AES-256-GCM encrypt/decrypt core
/internal/keystore   - keyfile + counter create/load/locate/export/import
/internal/fileformat - .mlp header read/write
```

## Backlog (post-v0.1, not open questions — deliberately deferred)
Scheduled into versions — see [ROADMAP.md](ROADMAP.md) for v0.2–v0.9 (batch
mode, key rotation, `mlp info`, AUR release, `--force` + GUI, timestamp
preservation, compression (all done), docs, tests).

Unscheduled:
- Streaming/chunked AEAD for very large files
- Password-based mode — rejected permanently, not revisited

## Open questions
None — all resolved for v0.1.
