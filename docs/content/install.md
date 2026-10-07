---
title: Install
weight: 1
---

## Arch Linux (AUR)

Three packages, all published automatically on every release:

| Package | What it is |
|---|---|
| `mlp` | The CLI, built from the tagged source. |
| `mlp-bin` | The CLI, as the prebuilt release binary. Faster to install; conflicts with `mlp`. |
| `mlp-gui` | The [desktop app](../gui). Separate package because it needs GL/X11 libraries the CLI doesn't. |

```sh
yay -S mlp        # or: yay -S mlp-bin
yay -S mlp-gui    # optional
```

The CLI packages also install the man pages (`man mlp`, `man mlp-encrypt`, …)
and bash, zsh and fish completions.

## Prebuilt binaries

Each [GitHub release](https://github.com/EldinBegano/mask-decryption/releases)
has archives for Linux, macOS and Windows on amd64 and arm64. Every archive
contains the `mlp` binary, `LICENSE`, shell completions under `completions/`,
and man pages under `man/`. Check the download against `checksums.txt` from
the same release:

```sh
sha256sum --check --ignore-missing checksums.txt
```

The desktop app isn't part of the release archives; it's Linux-only, via the
AUR or a source build.

## From source

Needs Go 1.23 or newer.

```sh
git clone https://github.com/EldinBegano/mask-decryption
cd mask-decryption
go build -o mlp ./cmd/mlp
```

A plain `go build` reports its version as `dev`. Release builds set it with
`-ldflags "-X main.version=<version>"`.

### The desktop app

`mlp-gui` uses [Fyne](https://fyne.io), so it needs CGO and the system GL
and X11/Wayland development headers. On Debian or Ubuntu:

```sh
sudo apt-get install libgl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev
go build -o mlp-gui ./cmd/mlp-gui
```

### Completions and man pages

```sh
mlp completion bash > ~/.local/share/bash-completion/completions/mlp   # also: zsh, fish, powershell
mlp gendoc ./man                                   # one man page per command
```

`gendoc` is hidden from `--help` because it's a build-time tool; it's what the
release pipeline uses to produce the shipped man pages.

## Check it works

```console
$ mlp --version
mlp 0.8.1
```
