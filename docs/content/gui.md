---
title: mlp-gui
weight: 7
---

A desktop app for the same keyfile and `.mlp` format as the CLI. It's a thin
front end over the same code, so files, keys and behavior are identical:
anything encrypted in one decrypts in the other.

```sh
yay -S mlp-gui
```

Linux only for now, packaged separately from the CLI because it needs system
GL and X11/Wayland libraries. To build it yourself, see
[Install](../install#the-desktop-app).

## Using it

1. **Pick a target**: *Choose file…*, *Choose folder…*, or drag and drop
   (if you drop several items, the first one is used).
2. **Encrypt or Decrypt.** The buttons follow what you picked: a plain file
   can only be encrypted, a `.mlp` file only decrypted, a folder either.
3. **Watch the results.** Each file appears in the list as it finishes, then
   a summary: `N encrypted, M skipped, K failed`.

Folders follow the same rules as the CLI's
[batch mode](../commands/encrypt#a-directory).

## Dialogs

- **Replace existing files?** If some outputs already existed, the app lists
  them and offers to replace them. This is the GUI's `--force`: *Yes* redoes
  just those files, *No* leaves them alone.
- **New keyfile.** The first encrypt creates the keyfile; a dialog says where
  it is and that you should back it up.
- **No keyfile.** Decrypting without one explains the situation and points to
  *Import backup*.

## Keyfile button

Shows where the keyfile is, with two actions:

- **Export backup…** writes the keyfile and counter to a folder you choose
  and shows the SHA-256, same as
  [`mlp keyfile export`](../commands/keyfile-export). It asks before
  replacing a different key's backup.
- **Import backup…** installs a backup, asking first if a key already exists,
  same as [`mlp keyfile import`](../commands/keyfile-import).

The button is disabled while a run is in progress.

## CLI-only

`rotate`, `keygen`, `verify` and `info` are deliberately left out of the
app. Use the CLI for those.

`mlp-gui --version` prints the version without opening a window.
