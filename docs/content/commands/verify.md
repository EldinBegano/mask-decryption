---
title: mlp verify
linkTitle: verify
weight: 3
---

```text
mlp verify <file.mlp>
```

Checks that a `.mlp` file is intact and decrypts under the active key,
without writing anything to disk. The file is decrypted in memory and the
result thrown away.

```console
$ mlp verify notes.mlp
OK: notes.mlp is intact
$ mlp verify damaged.mlp
error: damaged.mlp: authentication failed: data is corrupted, tampered with, or was encrypted with a different key
```

Useful after copying `.mlp` files to a backup drive, or before deleting the
originals.

Unlike [`info`](../info), `verify` needs the keyfile: without the key there's
no way to check the authentication tag. A file from mlp v0.6.0 or v0.7.0
passes with a [note](../decrypt#tampered-or-corrupted-files) that its header
isn't tamper-protected.

| Situation | Exit |
|---|---|
| Intact | 0 |
| Header unreadable (not an mlp file, truncated, unknown version) | 1 |
| No keyfile | 2 |
| Name doesn't end in `.mlp` | 5 |
| Authentication failed | 6 |

`verify` takes one file. To check a folder, loop over it:

```sh
find ~/backup -name '*.mlp' -exec mlp verify {} \;
```
