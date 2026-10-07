---
title: mlp info
linkTitle: info
weight: 4
---

```text
mlp info <file.mlp>
```

Shows what a `.mlp` file's header says about it. Reads only the header: no
key needed, nothing decrypted, nothing created.

```console
$ mlp info notes.mlp
file:            notes.mlp
format version:  2
original ext:    txt
decrypts to:     notes.txt
stored size:     28 bytes
modified:        2026-10-07T12:10:08+02:00
accessed:        2026-10-07T12:10:08+02:00
```

| Field | Meaning |
|---|---|
| `format version` | The file's [header version](../../concepts/file-format), `1` or `2`. |
| `original ext` | The stored extension, or `(none)`. |
| `decrypts to` | The name `mlp decrypt` would write, without `-o`. |
| `stored size` | Size of the encrypted payload, without header and tag. For a [compressed](../../concepts/compression) file that's the compressed size: `info` can't know the original size without decrypting. |
| `modified`, `accessed` | Stored timestamps in local time (RFC 3339). Files without them show `timestamps:      (not stored)`. |

{{< callout type="info" >}}
`info` can't tell whether a file is intact or was tampered with, since that
takes the key. Use [`mlp verify`](../verify) for that.
{{< /callout >}}

| Situation | Exit |
|---|---|
| Header read | 0 |
| Not an mlp file, unsupported version, truncated, or a directory | 1 |
| Name doesn't end in `.mlp` | 5 |
