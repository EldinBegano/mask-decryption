---
title: mlp encrypt
linkTitle: encrypt
weight: 1
---

```text
mlp encrypt <file|dir> [-o output] [-f]
```

Encrypts a file into a `.mlp` file next to it, or every file under a
directory.

| Flag | Meaning |
|---|---|
| `-o`, `--output <path>` | Write to this path instead of the default. Single file only. |
| `-f`, `--force` | Replace an existing output file instead of failing. |

## A single file

```console
$ mlp encrypt notes.txt
encrypted -> notes.mlp
$ mlp encrypt notes.txt -o notes-2026.mlp
encrypted -> notes-2026.mlp
```

The extension is replaced with `.mlp`, and the original one is stored in the
header so `decrypt` can restore it. A file with no extension, including
dotfiles like `.env`, gets `.mlp` appended: `.env` → `.env.mlp`.

The first encrypt on a machine creates the keyfile and says so:

```text
WARNING: new keyfile created at /home/you/.config/mask-decryption/keyfile
         Back it up now: mlp keyfile export <path>
         If it's lost, every file encrypted with it becomes permanently unreadable.
```

Along with the contents, `encrypt` stores the file's permission bits,
modification time and access time; `decrypt` restores all three. Contents
are [compressed](../../concepts/compression) first when that helps. With
`-v` you can see the effect:

```console
$ mlp encrypt -v notes.txt
encrypted -> notes.mlp
  input:  2800 bytes
  output: 44 bytes
```

(`output` is the encrypted payload; the file on disk also has its
[header](../../concepts/file-format).)

## A directory

```console
$ mlp encrypt taxes
encrypted -> taxes/2025.mlp
encrypted -> taxes/2025.txt.mlp
encrypted -> taxes/receipts.mlp
3 encrypted, 0 skipped, 0 failed
```

Every regular file under the directory, recursively, is encrypted to its own
`.mlp` beside it, following the single-file rules.

- **Hidden files and folders are included.**
- **Files already ending in `.mlp` are skipped**, not double-encrypted. They
  count as "skipped" in the summary; `-v` lists them.
- **Symlinked files are followed; symlinked folders aren't entered**, so the
  walk can't loop or leave the tree.
- Pipes, sockets and devices are skipped, and so is `mlp`'s own config
  directory if it happens to be inside.
- **Name collisions within one run** fall back to appending: above,
  `2025.pdf` took `2025.mlp`, so `2025.txt` became `2025.txt.mlp`. Both decrypt
  to their original names. Only files created in the same run trigger this; an
  output that already existed beforehand is an error, so re-running on a
  folder never creates duplicates.
- **One failure doesn't stop the run.** Each error is printed as it happens,
  then the summary. The exit code is the failures' shared
  [code](../../scripting#exit-codes) if they all agree, otherwise 1.

`-o` can't be combined with a directory.

## Errors

| Situation | Message | Exit |
|---|---|---|
| Output already exists | `error: output file notes.mlp already exists` | 4 |
| Input is already `.mlp` | `error: notes.mlp is already a .mlp file` | 5 |
| Output is the input itself | `error: input and output are the same file: notes.txt` | 1 |
| Output is a directory | `error: output path backup is a directory` | 1 |
| Output's folder doesn't exist | `error: open backup/.mlp-….tmp: no such file or directory` | 1 |

With `--force`, an existing output is replaced atomically, and the result
line says `(overwrote existing)`. The same-file and directory checks still
apply.
