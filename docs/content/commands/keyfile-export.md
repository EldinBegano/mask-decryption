---
title: mlp keyfile export
linkTitle: keyfile export
weight: 7
---

```text
mlp keyfile export <path> [-y]
```

Copies the keyfile and its nonce counter into the folder `<path>`, created if
needed. This is how you back up your key, and the only protection against
losing it.

| Flag | Meaning |
|---|---|
| `-y`, `--yes` | Replace a backup of a *different* key without asking. |

```console
$ mlp keyfile export /media/usb/mlp-backup
exported keyfile -> /media/usb/mlp-backup
sha256: 584e2222188015e3f327f27baf71737f21388b20f98ee0ece9ee90336bf5ec0a
```

- The folder gets two files, `keyfile` and `counter`. Keep them together.
- The SHA-256 is computed from the copy read back from `<path>`, so it
  confirms what actually landed there. `sha256sum <path>/keyfile` later
  should print the same.
- If `<path>` already has a backup of the **same** key, it's refreshed
  without asking.
- If it has a backup of a **different** key, `export` asks before replacing
  it, because that might be the other key's only copy.
- If the active counter is missing or corrupt, the backup gets a counter
  starting at a random, very high value (and a warning) instead of 0, so a
  restore can't repeat a nonce the key already used.

Run it after the first encrypt and after every `keygen` or `rotate`. See
[Back up your key](../../backups).

| Situation | Exit |
|---|---|
| Exported | 0 |
| Prompt declined, or a write error | 1 |
| No keyfile | 2 |
