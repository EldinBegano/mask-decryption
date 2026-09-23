# mask-decryption — Roadmap v0.2 → v0.9

Driver: v0.2–v0.3 are personal-use polish, v0.4 is the AUR release, v0.5
finishes the remaining features (`--force`, GUI), v0.6 preserves file
timestamps, v0.7 investigates shrinking `.mlp` output (like 7z), v0.8 is
docs, v0.9 is the automated test suite. Password-based mode is permanently
rejected — keyfile-only stays the design.

(v0.4 onward shifted by one when the AUR release was inserted as v0.4. Docs
and tests shifted again, from v0.6/v0.7 to v0.8/v0.9, to make room for
timestamp preservation and compression.)

## v0.2 — Batch mode + key rotation  (implemented; behavior specified in SPEC.md)

### `mlp encrypt <dir>` / `mlp decrypt <dir>`
- Recursive walk. Per-file `.mlp`, mirrored directory structure (not a
  single bundled archive) — each file follows the same rules as single-file
  encrypt/decrypt today: keep both, abort-per-file on existing output,
  preserve permission bits, symlinks followed.
- Encrypt: a file already ending in `.mlp` found inside the tree is
  **skipped** (noted in the end-of-run summary), not an error for the
  whole batch.
- One failing file doesn't stop the run: failures reported, summary at
  the end, non-zero exit.
- `notes.md` + `notes.txt` in one folder: the second falls back to
  `notes.txt.mlp` (only vs. files created in the same run).
- Hidden files included; symlinked dirs not descended; keystore dir skipped.
- Decrypt: walks the dir for `*.mlp`, restores each the same way.
- Prints an end-of-run summary: `N encrypted, M skipped`.

### `mlp rotate <file.mlp...|dir>`
Re-encrypts existing `.mlp` file(s) under a freshly generated key.
**All-or-nothing:**
1. Decrypt every target with the *current* key first, into memory/temp.
2. Generate a new key + reset counter (not yet committed as active).
3. Re-encrypt each target under the new key.
4. Only if every file in the batch succeeds: commit the new key+counter as
   active, then atomically replace each original `.mlp` with its
   re-encrypted version.
5. Any single failure (auth fail, IO error) aborts the whole operation —
   old key stays active, nothing on disk is touched.
6. The old key is kept as `keyfile.old` so a crash mid-swap is recoverable.

This becomes the recommended path for key rotation. `mlp keygen` alone
still exists for "I accept old files become unreadable."

## v0.3 — `mlp info`  (implemented; behavior specified in SPEC.md)
- `mlp info <file.mlp>` prints format version + original extension from
  the header, without touching the keyfile or ciphertext. Works even with
  no keyfile present.

## v0.4 — AUR release  (built; waiting on the release steps below)
Ships to the Arch User Repository as two packages: **`mlp`** (builds from the
tagged source) and **`mlp-bin`** (installs the prebuilt goreleaser binary).
`mlp-bin` `provides`/`conflicts` with `mlp`. Both install the binary, the GPL
license, and bash/zsh/fish completions.

Decided:
- Repo goes **public** (`makepkg` must download from a public URL).
- License **GPL-3.0-or-later**: `LICENSE` at the root, SPDX header in every
  `.go` file, `license` field in both PKGBUILDs.
- Package name `mlp`.
- Fully automated on every stable tag, with a **dedicated CI SSH key** (not
  the personal `~/.ssh/aur`), kept in the `AUR_SSH_KEY` GitHub secret.
- PKGBUILDs live in this repo under `packaging/aur/`.
- Completions installed by both packages.

Built:
- `mlp --version` (`main.version`, injected with `-X` by goreleaser and the
  `mlp` PKGBUILD; `dev` for a plain `go build`).
- `go.mod` directive lowered from `1.27.1` to `1.23` (verified to build and
  vet with Go 1.23.0).
- goreleaser: version ldflags, completions generated in a `before` hook,
  `LICENSE` + `completions/` in every archive.
- `packaging/aur/mlp/PKGBUILD` (source, follows Arch's Go guidelines: PIE,
  `-trimpath`, external linking; `check()` does a real encrypt/decrypt
  roundtrip and a version check) and `packaging/aur/mlp-bin/PKGBUILD`.
- `packaging/aur/publish.sh <pkg> <ver> [--push]`: rewrites `pkgver`, runs
  `updpkgsums`, regenerates `.SRCINFO`, builds with `makepkg` (running
  `check()`), runs `namcap` (fails on errors), then commits and pushes to the
  AUR. Re-running an already-published version is a no-op.
- `release.yml`: after goreleaser succeeds, an `aur` job (in an
  `archlinux:base-devel` container) publishes both packages. It pins the AUR
  host key to the fingerprint published on aur.archlinux.org and skips
  pre-release tags (`-` is illegal in a `pkgver`). A manual `workflow_dispatch`
  with `tag=vX.Y.Z` re-runs just the AUR step.
- Verified locally: both PKGBUILDs build, pass `check()`, and produce correct
  packages; `publish.sh` run end to end against local bare git repos.
  **Not verified:** `namcap` (not installed here; runs in CI), a clean-chroot
  build, and the real push to aur.archlinux.org.

Release steps (need you):
1. Commit and push this work.
2. Add the CI public key to your AUR account (My Account, SSH Public Key, as a
   new line/entry next to your existing key).
3. Make the repo public (`gh repo edit --visibility public
   --accept-visibility-change-consequences`). This exposes all history and
   every prior release, including the author email on the commits.
4. Tag `v0.4.0` and push the tag. goreleaser publishes the release, then the
   `aur` job creates the `mlp` and `mlp-bin` packages.
5. Check https://aur.archlinux.org/packages/mlp and `mlp-bin`, then
   `yay -S mlp` (or `mlp-bin`) and run a roundtrip.
6. If the `aur` job fails, fix it and re-run via Actions > Release > Run
   workflow with `tag=v0.4.0`.

Known limits:
- An AUR SSH key is per account, not per package: the CI key could also push
  to your other AUR packages (`franklyn-bin`, `franklyn-bin-dev`). It is
  separate from your personal key, so it can be revoked on its own.
- `mlp` `.SRCINFO` checksums come from GitHub's tag tarball; GitHub has
  changed archive bytes before, which would need a `pkgrel` bump.

## v0.5 — `--force` flag + GUI  (built; behavior specified in SPEC.md)

Decided:
- GUI scope: encrypt/decrypt of a file or folder, results list, overwrite
  prompt, first-run backup notice, keyfile export/import. `rotate`, `keygen`,
  `verify`, `info` stay CLI-only.
- Distribution: Linux-first. New AUR package `mlp-gui` (source build,
  installs a `.desktop` launcher and icon), published by the same release
  job. No macOS/Windows builds, no GUI asset on the GitHub release.

Built:
- `internal/ops`: file-level encrypt/decrypt, batch walk and atomic writes
  extracted out of `cmd/mlp`, so the CLI and GUI share one implementation.
  It never prints or exits; errors carry a `Kind` that the CLI maps to exit
  codes and the GUI maps to dialogs. The CLI's behavior is unchanged.
- `--force` / `-f` on `encrypt` and `decrypt` (single and batch): atomic
  replace via temp file + rename; refuses same-file and directory outputs.
- `cmd/mlp-gui` (Fyne 2.8, still `go 1.23`): pick or drop, live results,
  overwrite prompt, key notices, key export/import, `--version`.
- `packaging/aur/mlp-gui/PKGBUILD`; `release.yml` publishes it alongside
  `mlp-bin` and `mlp`; CI installs the GL/X11 dev libraries.
- `mlp`'s PKGBUILD now fetches only the CLI's dependencies
  (`go list -deps ./cmd/mlp`) instead of all of Fyne.

Verified: the 44-check v0.2/v0.3 regression script still passes after the
refactor; a new 31-check `--force` script passes; the GUI was driven
headless through Fyne's test driver against real files (encrypt, forced
retry, folder batch both ways, no-keyfile) and its screenshots reviewed;
the binary was started on a real Wayland desktop; `mlp` and `mlp-gui`
PKGBUILDs build, pass `check()` and `namcap` (no errors) in the CI Arch
image; the CI apt package list builds and vets in a Go 1.23 container.

**Not verified:** clicking through the GUI by hand (native file pickers,
drag-and-drop, the confirm dialogs), and the real AUR push of `mlp-gui`.

Release steps: commit and push; tag `v0.5.0`. The first `mlp-gui` push
creates the AUR package. If only the AUR step fails, re-run it with
Actions > Release > Run workflow, `tag=v0.5.0`.

Known limits:
- Dropping several items uses only the first.
- The GUI needs a display and GL; it can't be checked in a headless build,
  so the AUR `check()` only runs `--version` and validates the `.desktop`
  file.
- Wayland drag-and-drop support depends on the GLFW backend; the pickers
  always work.

## v0.6 — Preserve file timestamps  (built; behavior specified in SPEC.md)
Encrypt/decrypt previously only carried over permission bits (size grows by
the header+tag overhead, owner was never copied, atime/mtime/ctime were not
preserved). This adds mtime and atime.

Decided:
- Both mtime and atime, not mtime-only.
- If restoring the timestamp on decrypt fails (odd filesystem, permission
  quirk), warn loudly and keep the decrypted file — matches the existing
  nonce-fallback pattern, metadata failing doesn't fail the operation.
- `mlp info` shows the stored timestamp.

Built:
- `fileformat.Version` bumped to `0x02`: adds a flags byte (bit 0 =
  timestamps follow), and, when set, 12-byte mtime + 12-byte atime (int64
  unix seconds + uint32 nanoseconds) after the nonce. `ReadHeader` still
  accepts `0x01` (no flags byte, no timestamps); `WriteHeader` always
  writes `0x02`. A `0x02` header can still have no timestamps (flag unset)
  — used when re-encrypting a file whose original timestamps aren't known,
  see `mlp rotate` below, so nothing is fabricated.
- `internal/ops`: `EncryptFile`/`EncryptDir` read `info.ModTime()` and a new
  `accessTime(info)` helper, and write both into the header.
  `DecryptFile`/`DecryptDir` call `os.Chtimes` after writing and `Chmod`;
  failure sets `Result.TimestampFailed` (and `BatchResult.TimestampFailed`
  for a batch) instead of failing the operation — the CLI and GUI both
  print/show a warning for it, same pattern as `NonceFallback`.
- Access time isn't exposed portably by `os.FileInfo`, so
  `internal/ops/atime_linux.go` / `atime_darwin.go` read it from
  `syscall.Stat_t` (field name differs: `Atim` on Linux, `Atimespec` on
  Darwin — a real bug caught before it shipped, a blanket `//go:build unix`
  file would have failed to compile on Darwin), `atime_windows.go` from
  `syscall.Win32FileAttributeData`, and `atime_other.go` falls back to
  `ModTime()` for any other OS (none is shipped).
- `mlp rotate` (`cmd/mlp/rotate.go`) carries `hdr.ModTime`/`hdr.AccessTime`
  from the file it read into the new header it writes, so rotating doesn't
  reset a file's timestamp to "now" — and doesn't invent one for a v1 file
  that never had one.
- `mlp info` prints `modified:`/`accessed:` (RFC 3339, local time) when
  present, else `timestamps:      (not stored)`. Also fixed a latent bug
  it printed the package's current-write version constant instead of the
  file's actual parsed version — harmless while only one version existed,
  wrong the moment a second one did.

Verified: a 20-check script covering plain and batch encrypt/decrypt exact
mtime+atime roundtrip, `mlp info` timestamp display, `mlp rotate` carrying
the stored timestamp through (not resetting it), and — the one that
mattered most — a hand-built version-1-shaped `.mlp` (real ciphertext and
nonce from a real encrypted file, header bytes reassembled by hand into the
old, shorter layout) still decrypts correctly and gets a fresh timestamp,
proving old files stay readable. Also checked: an unknown flag bit doesn't
crash `info` or `decrypt`. Full build + `go vet` on the default toolchain,
on the Go 1.23.0 minimum, and cross-compiled for linux/darwin/windows.

Known limits:
- Owner/group and ctime are still never touched (ctime can't be set on
  Linux regardless).
- The header is unauthenticated (true of the extension field since v0.1
  too): a tampered timestamp isn't caught by `verify`.

## v0.7 — Shrink `.mlp` output  (built; behavior specified in SPEC.md)
Started exploratory — "look if we can make the file a little smaller like
7z" — and landed as a real feature.

Key constraint: **compression happens before encryption, on the
plaintext.** AES-GCM ciphertext is high-entropy and doesn't compress —
compressing the `.mlp` output after the fact would gain nothing. So this
is compress-then-encrypt, decrypt-then-decompress.

Decided:
- Algorithm: zstd, via `klauspost/compress/zstd`. Pinned to **v1.18.4**,
  not `@latest` — v1.19+ needs Go 1.24, and later v1.18.x patches (`.5`
  onward) need 1.24 too; v1.18.4 is the newest one that still only needs
  1.23, matching the floor already promised in SPEC.md. `go mod tidy`
  re-resolved this to latest twice while wiring the import in; both times
  it was caught by re-checking `go.mod` after tidying, not assumed.
- Policy: automatic, always try, keep the compressed form only if it's
  smaller — no flag. Matches 7z's own store-vs-deflate choice per file.
- `mlp info`: no compression reporting (ratio, algorithm) added.

Built:
- No new `fileformat` version: v0.6 already introduced an extensible flags
  byte, so compression reuses it as bit 1 (`flagCompressed`) instead of
  needing a version bump to `0x03` as originally guessed above.
- **Real gap found and fixed while designing this:** `ReadHeader` (v0.6)
  silently ignored any flag bit it didn't recognize. That's fine as long
  as no second bit exists — but the moment `flagCompressed` shipped, an
  old v0.6 binary opening a v0.7 file would decrypt successfully and
  silently write out still-compressed garbage, no error. Fixed by having
  `ReadHeader` reject any version-`0x02` header with an unrecognized flag
  bit (`ErrUnknownFlag`). This protects v0.7+ binaries against a future
  v0.8+ flag the same way; it cannot retroactively patch v0.6 binaries
  already installed — documented as a known gap of that release.
- `internal/ops/compress.go`: `maybeCompress`/`decompress`, one-shot
  `EncodeAll`/`DecodeAll` (matches the project's whole-file, non-streaming
  model). `EncryptFile` compresses before `crypto.Encrypt` if it shrinks
  the data; `DecryptFile` decompresses after `crypto.Decrypt` when the
  header says to. Batch and `--force` needed no changes — they already go
  through these two functions.
- **Second real bug found and fixed:** `mlp rotate` re-encrypts by calling
  `crypto.Decrypt`/`crypto.Encrypt` directly, not through
  `internal/ops`, and built its own new `fileformat.Header`. It already
  needed fixing once for v0.6 (to carry timestamps through) and needed the
  same fix again here — it wasn't copying `Compressed` into the new
  header, which would have re-encrypted already-compressed bytes under a
  header claiming they weren't compressed: correct ciphertext, wrong flag,
  silent corruption on the next decrypt.
- `mlp info`'s size field renamed `plaintext size:` → `stored size:`: for
  a compressed file the old label was actively wrong (it reported the
  compressed size while claiming to be the original), and `info` has no
  way to learn the true original size without the key. This is a
  correctness fix, not new compression reporting — it doesn't say whether
  compression was used, just stops overclaiming about the number it
  already showed.

Verified: the existing 20-check v0.6 regression script still passes,
including the section that (correctly) started giving an unknown-flag
error at an earlier step than before, since the "unknown flag = reject"
guard now fires. A new 14-check script covers: a highly compressible file
lands well under its own plaintext size; random/incompressible data adds
only the fixed header+tag overhead (no penalty); already-gzipped input
doesn't grow; a mixed batch (compressible + incompressible together)
round-trips correctly; rotating a compressed file stays small after
rotation instead of re-inflating, and its content is still correct after;
a hand-set future flag bit is rejected by both `info` and `decrypt`, with
no output file written. Full build + `go vet` on the default toolchain,
the Go 1.23.0 minimum, and cross-compiled for linux/darwin/windows.

Known limits:
- No streaming: whole file loaded into memory for compression too, same
  as encryption (see "Large files").
- `mlp v0.6` binaries cannot safely decrypt a `.mlp` file compressed by
  v0.7+ — see "real gap found and fixed" above.
- Compression level is zstd's default speed/ratio tradeoff
  (`SpeedDefault`); not configurable, not asked about.

### Addendum: found and fixed a third, more serious bug during this work — header tampering could silently corrupt decrypted output, not just metadata

Demonstrated by hand, not just reasoned about: encrypted a 180KB repetitive
file (genuinely compressed to 148 bytes), flipped the compressed flag off in
the header with a text editor — no key needed, header bytes were never
covered by the GCM tag — and ran `mlp decrypt`. It printed `decrypted ->
big_out.txt` and exited 0. The 180KB original became an 86-byte file of raw
zstd bytes. A clean "success" over silently wrong file content, not just a
wrong filename or timestamp, and triggerable by anyone (or anything — a sync
conflict, bit rot on that exact bit) with only write access to the `.mlp`
file, no key required.

Root cause: extension, timestamps, and now the compressed flag were
metadata *about* the ciphertext, but never bound to it. GCM's tag only ever
covered the ciphertext itself.

Fixed by using AES-GCM's own additional-authenticated-data (AAD) mechanism:
`crypto.Encrypt`/`Decrypt` gained an explicit `aad []byte` parameter, and
`fileformat.Header.AAD(headerBytes)` says what to pass — the header's own
encoded bytes for a version `0x02` header (binding it to its ciphertext), or
nil for a version `0x01` header (which predates this and always used nil).
`fileformat.EncodeHeader`/`ReadHeaderCapture` were added so both sides
compute identical bytes: the writer encodes once and uses the same bytes
for the AAD and the on-disk write; the reader captures the exact bytes it
parsed (via `io.TeeReader`) rather than re-serializing the parsed struct,
avoiding any risk of the two representations drifting apart.

This closes the gap for `flagCompressed`, but also, as a side effect, for
the extension and timestamp fields — tampering with any part of a v2
header now fails loudly (`ErrAuthFailed`) instead of silently restoring a
wrong filename, wrong date, or (compression's case) wrong file content.
`SPEC.md`'s file format section was wrong the moment this landed ("none of
the header is authenticated") and has been corrected.

Zero backward-compatibility cost: format version `0x02` (the one with a
flags byte at all) was never tagged or released — only v0.1 through v0.5
were, and those are all version `0x01`, unaffected. Verified with a
genuinely reconstructed version-1 file (real AAD=nil ciphertext built by
calling `crypto.Encrypt` directly, exactly as a real pre-v0.6 binary would
have, not by truncating a v2 file) — it still decrypts correctly. Also
verified: a real v1 file with its version byte changed to claim `0x02`
fails to decrypt (its actual tag has no AAD binding, so pretending it does
doesn't help an attacker either). Repeated the original bug's exact repro
after the fix: same tampered file now fails with `ErrAuthFailed`, exit 3,
no output file written.

Rechecked after the fix: the 20-check v0.6 script (now 22, with the v1
compat test rebuilt as above) and the 14-check v0.7 script both still pass
in full; build + `go vet` clean on the default toolchain, the Go 1.23.0
minimum, and cross-compiled for linux/darwin/windows.

### Addendum 2: full code review turned up two more issues, both fixed

Prompted by "look for more bugs," a fresh read of every package (not just
the AAD area) surfaced three findings; two were fixed, one (below) wasn't
asked for and is left as a documented gap.

**Fixed — `mlp keygen` and `mlp keyfile import` destroyed the active key
with no way back.** Demonstrated directly: `mlp keygen -y` (or `import`
pointed at the wrong path) overwrote the keyfile in place with nothing
preserved — `mlp rotate` was the only one of the three key-replacing
operations that kept a `keyfile.old` safety net. One mistake was
permanent, unrecoverable data loss, for an encryption tool, with the fix
already proven elsewhere in the same codebase.

Fixed by applying `Rotation.Commit`'s pattern to both: `keystore.Keygen`
and `keystore.Import` now back up whatever key is currently active (via a
new shared `backupIfExists` helper, same `writeFileAtomic` underneath) to
`keyfile.old` before replacing it. Single slot, most-recent-previous only
— not a history, matching `rotate`'s existing model. Nothing is backed up
on a genuinely first-ever key (nothing to preserve).

- `Keygen` no longer resets the counter to 0 either: like `rotate`, it's a
  never-lowered high-water-mark across every key this config dir has ever
  had, so if `keyfile.old` is restored by hand later, resuming under it
  can't reuse a nonce it already used. A first-ever key still starts the
  counter at 0 (readCounter fails, same fallback as before).
- `Import` backs up the key **and its matched counter** together
  (`keyfile.old` + a new `counter.old`) — it fully replaces the counter
  too (unlike `Keygen`), so only the key alone wouldn't have been safe to
  resume from.
- New `keystore.OldCounterPath`, alongside the existing `OldKeyPath`.
- CLI (`mlp keygen`, `mlp keyfile import`) and GUI (Keyfile > Import
  backup) both updated: confirmation prompts no longer say
  "permanently unreadable" (no longer true), success output/dialogs say
  where the backup landed, matching what `mlp rotate` already prints.

**Fixed — `DecryptFile` checked "does the output already exist" only
*after* paying for the full key fetch, decrypt and decompress; `EncryptFile`
checked first.** Demonstrated directly: no keyfile present *and* the
output already existed → reported "keyfile not found" (exit 2), hiding
that the output conflict (exit 4) was the real, separate blocker, only
discovered on a wasted retry once a keyfile existed. Not data-loss
(`decrypt` only ever calls `LoadKey`, never auto-creates), but a genuine,
reproducible wrong-priority error.

Fixed by moving the output-path computation and `checkOutput` call to
right after the header parses (all it needs is `hdr.Ext`) and before
`getKey`/decrypt/decompress — mirrors `EncryptFile`'s existing "cheap
validation before expensive/key-dependent work" order, and matches the
`KeyFunc` doc comment's stated intent, which `DecryptFile` alone didn't
follow.

**Fixed — `Rotation.Commit` silently treated a corrupted/missing nonce
counter as 0, with no warning.** Same corrupted counter file, two code
paths: a normal `encrypt` (`NextNonce`) printed `WARNING: nonce counter
state was missing or corrupt — using random nonces instead.`; `rotate`'s
`Commit` (identical failure) said nothing at all and just reset the
counter. Verified directly — same file, side-by-side. Low practical
severity on its own: turning this into an actual nonce reuse needs
corruption *and* a rotate *and* someone later manually restoring
`keyfile.old`, a narrow multi-step chain — but a real inconsistency
between two code paths that otherwise handle the identical failure the
same way everywhere else.

Fixed by giving `Commit` the same `fellBack` signal `NextNonce` already
returns: it now reports whether the counter had to be treated as 0, and
`cmd/mlp/rotate.go` prints a loud warning when that happens, explaining
the actual risk precisely — decrypting with a later-restored `keyfile.old`
is still safe (decryption doesn't consume nonces), but encrypting *new*
files under it isn't, since its usage before this rotation is no longer
tracked. Verified both directions directly: a healthy counter prints no
warning at all; a corrupted one warns with the guidance above, and the
rotated file still decrypts correctly either way.

Verified: a new 21-check script covers the keygen/import fixes above —
`keyfile.old` created and byte-correct after `keygen`, unreadable under
the new key, readable again once restored by hand; a second `keygen`
backs up the *second* generation, not the first (proving single-slot, not
history); counter never lowered by `keygen`, but still starts at 0 on a
genuinely first-ever key; `keyfile import` produces a matched
`keyfile.old`+`counter.old` pair that together (not the key alone) restore
a working state; neither operation falsely claims a backup when there was
nothing to back up; the error-priority fix reproduces the exact scenario
above now returning exit 4, not 2, while a decrypt with no *output*
conflict still correctly reports exit 2 when the keyfile really is the
only problem; normal decrypt and `--force` decrypt both still work under
the new ordering. Full rebuild of all prior regression scripts
(44+31+22+14 checks, plus the `Commit` warning checks) still passes; build
+ `go vet` clean on the default toolchain, the Go 1.23.0 minimum, and
cross-compiled for linux/darwin/windows.

## v0.8 — Docs  (was v0.6)
- `README.md`: install (including `yay -S mlp`), quick start, full command
  reference, the "no recovery if keyfile is lost" warning stated up front.
- Man page: generated and shipped via goreleaser alongside release
  binaries, and installed by the AUR packages.

## v0.9 — Automated test suite  (was "beyond v0.6")
- Unit tests for `internal/crypto`, `internal/fileformat`, `internal/keystore`,
  `internal/ops`.
- Fuzz testing on `.mlp` header parsing (now more important: two format
  versions and a compression flag to fuzz by v0.9).
- Wire into CI (`go test ./...`, alongside the existing `go build`/`go vet`).

## Beyond v0.9 (explicitly not in this roadmap)
- Streaming/chunked AEAD for very large files — unscheduled.
- Password-based mode — rejected permanently, not revisited.
- Homebrew — investigated after v0.4 and dropped (repo doesn't meet
  homebrew-core's popularity bar; a personal tap means either a
  hand-written source formula or a cask needing a notarized binary or an
  `xattr` Gatekeeper-bypass hack). AUR is the only distro channel.
