---
title: mlp keygen
linkTitle: keygen
weight: 6
---

```text
mlp keygen [-y]
```

Generates a new random key and makes it active.

| Flag | Meaning |
|---|---|
| `-y`, `--yes` | Skip the confirmation prompt. |

You rarely need this: the first `mlp encrypt` creates a key on its own. And
to move existing files to a new key, use [`mlp rotate`](../rotate) instead,
which keeps them readable.

```console
$ mlp keygen
A keyfile already exists. Regenerating it means .mlp files encrypted with the old key can no longer be decrypted unless you restore it (the old key is kept as keyfile.old). Continue? [y/N]: y
new keyfile created at /home/you/.config/mask-decryption/keyfile
previous key kept at /home/you/.config/mask-decryption/keyfile.old — delete it once you no longer need it
WARNING: back it up now with 'mlp keyfile export <path>' — if it's lost, encrypted files become permanently unreadable.
```

If a key already exists, `keygen` asks first and keeps it as `keyfile.old`,
so an accidental `keygen` is recoverable by hand (see
[Recovering keyfile.old](../../backups#recovering-keyfileold-by-hand)).
`keyfile.old` is a single slot, not a history: a second `keygen` replaces it
with the key the first one created.

The nonce counter isn't reset, so a restored older key can't repeat a nonce
it used before.

| Situation | Exit |
|---|---|
| New key created | 0 |
| Prompt declined or unanswered | 1 |
