---
title: mlp decrypt
linkTitle: decrypt
weight: 2
---

```text
mlp decrypt <file.mlp|dir> [-o output] [-f]
```

Decrypts a `.mlp` file, or every `.mlp` file under a directory.

| Flag | Meaning |
|---|---|
| `-o`, `--output <path>` | Write to this path instead of the original name. Single file only. |
| `-f`, `--force` | Replace an existing output file instead of failing. |

## A single file

```console
$ mlp decrypt notes.mlp
decrypted -> notes.txt
$ mlp decrypt notes.mlp -o /tmp/peek.txt
decrypted -> /tmp/peek.txt
```

The original extension comes back from the header, along with the permission
bits and, if stored, the modification and access times. If the timestamps
can't be set (some filesystems refuse), the file is still written and you get
a warning:

```text
WARNING: notes.txt: could not restore the original timestamp (file itself is fine)
```

Use `mlp info` to see what a file will decrypt to without the key.

## A directory

```console
$ mlp decrypt taxes
decrypted -> taxes/2025.pdf
decrypted -> taxes/2025.txt
decrypted -> taxes/receipts
3 decrypted, 0 skipped, 0 failed
```

Every `.mlp` file under the directory, recursively, is decrypted beside
itself; other files are ignored. Walk rules, the summary and exit codes work
as for [`encrypt`](../encrypt#a-directory). The `.mlp` files are kept.

## Tampered or corrupted files

```console
$ mlp decrypt notes.mlp
error: notes.mlp: authentication failed: data is corrupted, tampered with, or was encrypted with a different key
```

Exit 3, and nothing is written. That covers a changed byte anywhere in the
contents or the header, a truncated file, and a file encrypted under a
different key. `mlp` can't tell those apart, and never writes output it
couldn't authenticate.

Files from mlp v0.6.0 and v0.7.0 decrypt normally but print a note, since
their headers weren't tamper-protected yet:

```text
note: old.mlp was made by mlp v0.6.0 or v0.7.0, whose headers aren't tamper-protected; re-encrypt it to add that
```

See [Compatibility](../../compatibility).

## Errors

| Situation | Message | Exit |
|---|---|---|
| No keyfile | `error: keyfile not found — run 'mlp encrypt' or 'mlp keygen' first` | 2 |
| Wrong key, corrupted or tampered | `error: …: authentication failed: …` | 3 |
| Output already exists | `error: output file notes.txt already exists` | 4 |
| Input isn't `.mlp` | `error: notes.txt is not a .mlp file` | 5 |

An existing output is reported (exit 4) before the key is even loaded, so a
missing keyfile doesn't hide it.
