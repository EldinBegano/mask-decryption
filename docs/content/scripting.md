---
title: Scripting
weight: 6
---

`mlp` is built to be called from scripts: plain-text output, stable exit
codes, and prompts that fail safe when nobody's there to answer.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. |
| 1 | Generic error, or a declined/unanswered prompt. |
| 2 | Keyfile missing. |
| 3 | Authentication failed: tampered, corrupted, or wrong key. |
| 4 | Output file already exists. |
| 5 | Wrong file type: input already `.mlp` (encrypt), or not `.mlp` (decrypt, verify, info, rotate). |
| 6 | `mlp verify` failed. |

**Batch runs** (`encrypt` or `decrypt` on a directory) exit 0 only if every
file succeeded. Otherwise they exit with the failures' shared code if they
all agree, or 1 if they're mixed. Re-running an encrypt on a folder that's
already done, for example, exits 4.

## Prompts

`rotate`, `keygen`, `keyfile import`, and `keyfile export` over a different
key's backup ask `[y/N]` on stderr and read the answer from stdin. In a
script:

- **No input** (stdin closed, `</dev/null`, a cron job) counts as *no*: the
  command prints `error: aborted, nothing was changed` and exits 1.
- **`-y`** skips the prompt.
- Piping an answer works too: `echo y | mlp keygen`.

## Output streams

| Stream | What goes there |
|---|---|
| stdout | Results: `encrypted -> notes.mlp`, batch summaries, `info` fields, `OK: … is intact`, export hashes. |
| stderr | `error:` lines, `WARNING:` and `note:` lines, prompts. |

No color codes are ever printed.

## Examples

Encrypt, verify, then remove the original only if all went well:

```sh
mlp encrypt report.pdf && mlp verify report.mlp && rm report.pdf
```

Treat "already encrypted" as fine when re-running over a folder:

```sh
mlp encrypt ~/vault
case $? in
  0|4) ;;                        # done; any failures were only outputs that already existed
  *)   echo "encrypt failed" >&2; exit 1 ;;
esac
```

Check every file in a backup:

```sh
find /mnt/backup -name '*.mlp' -print0 |
  xargs -0 -n1 mlp verify || echo "some files failed verification" >&2
```

Run against a separate keyfile, for tests or a portable setup:

```sh
export MLP_CONFIG_DIR=/media/usb/mlp
mlp decrypt notes.mlp
```

## Concurrency

Running several `mlp` processes at once is safe, including the CLI next to
`mlp-gui`: they take turns on the keyfile and counter through a lock file, so
no two encrypts get the same nonce. See
[Running several at once](../concepts/how-it-works#running-several-at-once).
