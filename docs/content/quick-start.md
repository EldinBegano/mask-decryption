---
title: Quick start
weight: 2
---

{{% steps %}}

### Encrypt a file

```console
$ mlp encrypt notes.txt
WARNING: new keyfile created at /home/you/.config/mask-decryption/keyfile
         Back it up now: mlp keyfile export <path>
         If it's lost, every file encrypted with it becomes permanently unreadable.
encrypted -> notes.mlp
```

The first encrypt creates the keyfile. The original `notes.txt` is kept;
`mlp` never deletes its input. Delete it yourself once you're happy.

### Back up the keyfile

Do this now, not later. A USB stick or any place that isn't this disk:

```console
$ mlp keyfile export /media/usb/mlp-backup
exported keyfile -> /media/usb/mlp-backup
sha256: 584e2222188015e3f327f27baf71737f21388b20f98ee0ece9ee90336bf5ec0a
```

Note the `sha256` line somewhere: it lets you confirm later that the backup
is still the right key. See [Back up your key](../backups) for details.

### Decrypt it

```console
$ mlp decrypt notes.mlp
decrypted -> notes.txt
```

The original name, extension, permission bits and timestamps come back from
the file's header. If `notes.txt` still exists, `mlp` refuses rather than
overwriting it; pass `-f` to replace it.

### Whole folders

```console
$ mlp encrypt taxes
encrypted -> taxes/2025.mlp
encrypted -> taxes/2025.txt.mlp
encrypted -> taxes/receipts.mlp
3 encrypted, 0 skipped, 0 failed
```

Every file under the folder, recursively, gets its own `.mlp` next to it.
Here `2025.pdf` took `2025.mlp`, so `2025.txt` fell back to `2025.txt.mlp`;
decrypting restores both original names. `mlp decrypt taxes` reverses it.

{{% /steps %}}

## Next

- [Commands](../commands): every flag and message.
- [How it works](../concepts): where the key lives and what's in a `.mlp` file.
- [Scripting](../scripting): exit codes for use in scripts.
