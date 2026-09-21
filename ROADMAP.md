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

## v0.6 — Docs  (was v0.5)
- `README.md`: install (including `yay -S mlp`), quick start, full command
  reference, the "no recovery if keyfile is lost" warning stated up front.
- Man page: generated and shipped via goreleaser alongside release
  binaries, and installed by the AUR packages.

## Beyond v0.6 (explicitly not in this roadmap)
- v0.7: automated test suite (unit + fuzz on `.mlp` header parsing).
- Streaming/chunked AEAD for very large files — unscheduled.
- Password-based mode — rejected permanently, not revisited.
- no Homebrew
