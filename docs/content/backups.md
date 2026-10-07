---
title: Back up your key
weight: 3
---

{{< callout type="error" >}}
The keyfile is the only way to decrypt your `.mlp` files. There is no
passphrase, no recovery key, no escrow. Lose the keyfile and its backups, and
the data is gone.
{{< /callout >}}

## When to back up

- Right after the first `mlp encrypt` (it prints a warning reminding you).
- After every `mlp keygen` or `mlp rotate`: both make a **new** key active,
  and your old backup doesn't decrypt files encrypted after that.
- After `mlp keyfile import` of a key you haven't backed up anywhere else.

## Export

```console
$ mlp keyfile export /media/usb/mlp-backup
exported keyfile -> /media/usb/mlp-backup
sha256: 584e2222188015e3f327f27baf71737f21388b20f98ee0ece9ee90336bf5ec0a
```

The folder is created if needed and ends up holding two files:

| File | What it is |
|---|---|
| `keyfile` | The 32-byte key. |
| `counter` | The nonce counter that goes with it (8 bytes). |

They travel together so that a restore carries on counting where the key left
off, rather than reusing a nonce it already used. See
[Keyfile and nonces](../concepts/how-it-works#nonces) for why that matters.

The printed SHA-256 is of the copy **as read back from the backup folder**, so
it describes what actually landed on the stick. Keep it somewhere separate.
To check a backup later:

```sh
sha256sum /media/usb/mlp-backup/keyfile
```

Exporting again to the same folder:

- **Same key:** the backup is refreshed silently.
- **A different key:** `mlp` asks first, since that may be the other key's
  only backup. `-y` replaces it without asking.

## Restore

```console
$ mlp keyfile import /media/usb/mlp-backup
keyfile imported
```

The backup is validated before anything changes: the folder must hold a
32-byte `keyfile` and an 8-byte `counter`, and can't be the config directory
itself. If a different key is active, `mlp` asks first and keeps that key as
`keyfile.old` (with its counter as `counter.old`), so importing the wrong
backup is recoverable.

## Recovering `keyfile.old` by hand

Every command that replaces the active key keeps the previous one next to it
as `keyfile.old`: `rotate`, `keygen` and `keyfile import`. It's a single slot,
"the previous key", not a history; each replacement overwrites it.

To go back to the previous key, swap the files in the
[config directory](../concepts/how-it-works#where-the-key-lives):

```sh
cd ~/.config/mask-decryption
mv keyfile keyfile.new         # keep the current key too, in case
mv keyfile.old keyfile
```

Leave `counter` where it is. `mlp` never lowers the counter when it replaces
a key, so the current counter is already at least as high as anything the old
key used, and encrypting under the restored key stays safe. (`counter.old`,
written by `import`, is there for completeness; you don't need it for this.)

The one exception: if `mlp rotate` warned that the counter state was missing
or corrupt, the old key's earlier usage is no longer tracked. Restoring it is
still fine for decrypting, but don't encrypt new files under it; rotate them
to a fresh key instead.

Back up `keyfile.new` if it decrypts anything you care about, or keep it
around until you're sure it doesn't.

## Moving to a new machine

1. On the old machine: `mlp keyfile export /media/usb/mlp-backup`.
2. On the new machine: `mlp keyfile import /media/usb/mlp-backup`.

Use export/import rather than copying `keyfile` by hand. A bare keyfile
decrypts fine, but with no `counter` beside it every encrypt falls back to a
random nonce and prints a warning, instead of carrying on the key's count.

## Portable setups

`MLP_CONFIG_DIR` overrides where `mlp` looks for its keyfile. Point it at a
folder on a USB stick to carry the key with you instead of installing it:

```sh
MLP_CONFIG_DIR=/media/usb/mlp mlp decrypt notes.mlp
```
