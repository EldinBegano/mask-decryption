---
title: mlp keyfile import
linkTitle: keyfile import
weight: 8
---

```text
mlp keyfile import <path> [-y]
```

Installs a backup made by `mlp keyfile export` as the active key.

| Flag | Meaning |
|---|---|
| `-y`, `--yes` | Skip the confirmation prompt. |

```console
$ mlp keyfile import /media/usb/mlp-backup
A keyfile already exists. Importing replaces it — files encrypted with the current key can no longer be decrypted unless you restore it (it's kept as keyfile.old, with counter.old, if you pointed this at the wrong backup by mistake). Continue? [y/N]: y
keyfile imported
previous key kept at /home/you/.config/mask-decryption/keyfile.old — delete it once you no longer need it
```

The prompt only appears when a key is already active. On a fresh machine,
`import` just installs the backup.

**Checked before anything changes.** The folder must contain a 32-byte
`keyfile` and a readable 8-byte `counter`, and must not be the config
directory itself. Anything else is refused with nothing touched.

**The previous key is kept.** If a different key was active, it's saved as
`keyfile.old` and its counter as `counter.old`, so importing the wrong backup
is recoverable. Re-importing a backup of the key that's already active leaves
`keyfile.old` alone, so it can't push out a different previous key.

**The counter is never lowered.** It becomes the higher of the backup's and
the current one.

| Situation | Exit |
|---|---|
| Imported | 0 |
| Prompt declined, invalid backup, or a write error | 1 |
