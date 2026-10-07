---
title: mlp documentation
linkTitle: Overview
# Every page uses the docs layout (sidebar, table of contents), the home page included.
cascade:
  type: docs
---

`mlp` encrypts and decrypts files with AES-256-GCM, using a single
auto-managed keyfile. There's no passphrase to remember and no key to type:
the first `mlp encrypt` creates a keyfile in your config directory, and every
later command finds it on its own.

```console
$ mlp encrypt notes.txt
encrypted -> notes.mlp
$ mlp decrypt notes.mlp
decrypted -> notes.txt
```

{{< callout type="warning" >}}
**There is no recovery mechanism.** If the keyfile is lost, every file
encrypted with it is permanently unreadable. Back it up as soon as it exists:
`mlp keyfile export <path>`. See [Back up your key](backups).
{{< /callout >}}

## Start here

{{< cards >}}
  {{< card link="install" title="Install" icon="download" subtitle="AUR packages, or build from source." >}}
  {{< card link="quick-start" title="Quick start" icon="sparkles" subtitle="Encrypt, back up the key, decrypt." >}}
  {{< card link="backups" title="Back up your key" icon="key" subtitle="Export, verify, restore, and recover keyfile.old." >}}
{{< /cards >}}

## Reference

{{< cards >}}
  {{< card link="commands" title="Commands" icon="terminal" subtitle="Every command, flag and message." >}}
  {{< card link="concepts" title="How it works" icon="cog" subtitle="Keyfile, nonces, file format, compression." >}}
  {{< card link="scripting" title="Scripting & exit codes" icon="code" subtitle="What scripts can rely on." >}}
  {{< card link="gui" title="mlp-gui" icon="desktop-computer" subtitle="The desktop app." >}}
  {{< card link="compatibility" title="Compatibility" icon="clock" subtitle="Which versions read which files." >}}
  {{< card link="troubleshooting" title="Troubleshooting" icon="question-mark-circle" subtitle="Error messages and what to do." >}}
{{< /cards >}}

## What mlp is not

- **Not password-based.** The key is a random 256-bit file, not derived from
  something you type. A password mode has been considered and permanently
  rejected.
- **Not multi-user.** One keyfile per config directory; there's no sharing or
  per-recipient keys.
- **Not streaming.** Each file is read into memory, compressed and encrypted
  in one go. Fine for personal files; not built for multi-gigabyte ones.
