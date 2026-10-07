---
title: Commands
weight: 4
sidebar:
  open: true
---

```text
mlp encrypt <file|dir> [-o output] [-f]        encrypt a file, or every file under a folder
mlp decrypt <file.mlp|dir> [-o output] [-f]    decrypt a .mlp file, or every .mlp under a folder
mlp verify <file.mlp>                          check integrity without writing anything
mlp info <file.mlp>                            show the header; no key needed
mlp rotate <file.mlp|dir>... [-y]              move files to a new key, all-or-nothing
mlp keygen [-y]                                start over with a new key
mlp keyfile export <path> [-y]                 back up the key and its counter
mlp keyfile import <path> [-y]                 restore a backup
mlp completion <bash|zsh|fish|powershell>      print a shell completion script
```

## Global flags

| Flag | Meaning |
|---|---|
| `-v`, `--verbose` | Print extra detail (sizes, skipped files). |
| `-h`, `--help` | Help for any command: `mlp encrypt --help`. |
| `--version` | Print `mlp <version>` and exit. |

## Output conventions

- Results go to stdout, one line per file: `encrypted -> notes.mlp`.
- Errors go to stderr, prefixed `error:`; warnings are prefixed `WARNING:`
  and notes `note:`.
- No colors, no progress bars, no telemetry.
- Prompts (`[y/N]`) read from stdin. With nothing to read, as in a script,
  the answer is no: the command exits 1 and changes nothing. Pass `-y`.

The same text is in the man pages, if installed: `man mlp`,
`man mlp-encrypt`, `man mlp-keyfile-export`, and so on.

{{< cards >}}
  {{< card link="encrypt" title="encrypt" icon="lock-closed" >}}
  {{< card link="decrypt" title="decrypt" icon="lock-open" >}}
  {{< card link="verify" title="verify" icon="badge-check" >}}
  {{< card link="info" title="info" icon="information-circle" >}}
  {{< card link="rotate" title="rotate" icon="refresh" >}}
  {{< card link="keygen" title="keygen" icon="key" >}}
  {{< card link="keyfile-export" title="keyfile export" icon="upload" >}}
  {{< card link="keyfile-import" title="keyfile import" icon="download" >}}
{{< /cards >}}
