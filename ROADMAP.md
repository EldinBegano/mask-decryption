# mask-decryption — Roadmap v0.2 → v0.6

Driver: v0.2–v0.3 are personal-use polish, v0.4 is the AUR release, v0.5
finishes the remaining features (`--force`, GUI), v0.6 is docs. Automated
tests are pushed to v0.7+ (not in this roadmap). Password-based mode is
permanently rejected — keyfile-only stays the design.

(v0.4 onward shifted by one when the AUR release was inserted as v0.4.)

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

## v0.4 — AUR release
Ship to the Arch User Repository as two packages: **`mlp`** (builds from the
tagged source) and **`mlp-bin`** (installs the prebuilt goreleaser binary).
`mlp-bin` `provides`/`conflicts` with `mlp`.

Decided:
- Repo goes **public** (AUR's `makepkg` must download source/binaries from a
  public URL; a private repo makes AUR impossible).
- License: **GPL-3.0-or-later**. Add `LICENSE`, set the PKGBUILD `license`
  field, drop "no license / all rights reserved" from SPEC.md.
- Package name `mlp` (matches the binary and the `.mlp` extension; free on
  AUR at time of writing, re-check before first push).

Work:
- `LICENSE` file (GPL-3.0 text) in the repo root.
- `mlp --version` (cobra `Version`, injected via goreleaser ldflags). Needed
  for AUR sanity checks and support requests; none exists today.
- Lower the `go` directive in `go.mod` to the true minimum. It currently
  says `1.27.1` only because that was the local toolchain; the source
  package builds offline in a chroot, so a directive newer than Arch's `go`
  would fail there. (SPEC already says 1.23+.)
- `PKGBUILD` for `mlp`: `makedepends=(go)`, builds from the release tag
  tarball with Arch's Go flags (`-trimpath`, PIE, `-buildid=`, no CGO).
- `PKGBUILD` for `mlp-bin`: installs the `linux_amd64` / `linux_arm64`
  archives from the GitHub release, checksums from `checksums.txt`.
- Install the GPL license into `/usr/share/licenses/mlp/`.
- Validate before pushing: `namcap`, `makepkg` in a clean chroot
  (`devtools`), and a real `pacman -U` install + `mlp` roundtrip.
- Publish: needs an AUR account with an SSH key; first push is manual.

Open (decide when v0.4 starts):
- Automation: goreleaser `aurs` / `aur_sources` pushing both PKGBUILDs on
  every tag (SSH key in a GitHub secret), vs. maintaining PKGBUILDs by hand.
- Shell completions (cobra generates bash/zsh/fish) installed by the
  packages — proposed, not confirmed.
- Where PKGBUILDs live (`packaging/aur/` in this repo vs. only in the AUR git).

## v0.5 — `--force` flag + GUI  (was v0.4)
- `--force` on `encrypt`/`decrypt`: overwrite an existing output file
  instead of aborting; still prints what it did.
- GUI (Fyne): thin wrapper over the same core library — encrypt / decrypt
  / batch through a file picker. Mirrors CLI behavior (keep both, no
  overwrite unless forced).
- Note: a Fyne GUI needs CGO and system GL/X11 libs, so it must be a
  **separate binary/package** (e.g. `mlp-gui`), not part of the CLI build.
  The CLI stays CGO-free and dependency-light, which keeps the AUR `mlp`
  package simple.

## v0.6 — Docs  (was v0.5)
- `README.md`: install (including `yay -S mlp`), quick start, full command
  reference, the "no recovery if keyfile is lost" warning stated up front.
- Man page: generated and shipped via goreleaser alongside release
  binaries, and installed by the AUR packages.

## Beyond v0.6 (explicitly not in this roadmap)
- v0.7: automated test suite (unit + fuzz on `.mlp` header parsing).
- Streaming/chunked AEAD for very large files — unscheduled.
- Password-based mode — rejected permanently, not revisited.
