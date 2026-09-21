# mask-decryption — Roadmap v0.2 → v0.5

Driver: v0.2–v0.4 are personal-use polish, v0.5 is docs. Automated tests are
explicitly pushed to v0.6+ (not in this roadmap). Password-based mode is
permanently rejected — keyfile-only stays the design.

## v0.2 — Batch mode + key rotation

### `mlp encrypt <dir>` / `mlp decrypt <dir>`
- Recursive walk. Per-file `.mlp`, mirrored directory structure (not a
  single bundled archive) — each file follows the same rules as single-file
  encrypt/decrypt today: keep both, abort-per-file on existing output,
  preserve permission bits, symlinks followed.
- Encrypt: a file already ending in `.mlp` found inside the tree is
  **skipped** (noted in the end-of-run summary), not an error for the
  whole batch.
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

This becomes the recommended path for key rotation. `mlp keygen` alone
still exists for "I accept old files become unreadable."

## v0.3 — `mlp info`
- `mlp info <file.mlp>` prints format version + original extension from
  the header, without touching the keyfile or ciphertext. Works even with
  no keyfile present.

## v0.4 — `--force` flag + GUI
- `--force` on `encrypt`/`decrypt`: overwrite an existing output file
  instead of aborting; still prints what it did.
- GUI (Fyne): thin wrapper over the same core library — encrypt / decrypt
  / batch through a file picker. Mirrors CLI behavior (keep both, no
  overwrite unless forced).

## v0.5 — Docs
- `README.md`: install, quick start, full command reference, the
  "no recovery if keyfile is lost" warning stated up front.
- Man page: generated and shipped via goreleaser alongside release
  binaries.

## Beyond v0.5 (explicitly not in this roadmap)
- v0.6: automated test suite (unit + fuzz on `.mlp` header parsing).
- Streaming/chunked AEAD for very large files — unscheduled.
- Password-based mode — rejected permanently, not revisited.
