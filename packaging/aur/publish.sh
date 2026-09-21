#!/usr/bin/env bash
# Update, validate and optionally push one AUR package from packaging/aur/<pkg>/.
#
#   publish.sh <pkg> <version> [--push]
#
# <version> has no leading "v". Must run as a non-root user (makepkg refuses
# root) with the makedepends installed. With --push it expects the AUR SSH key
# at ~/.ssh/aur and the AUR host key in ~/.ssh/known_hosts (set up by the
# release workflow), unless GIT_SSH_COMMAND is already set.
set -euo pipefail

pkg=${1:?usage: publish.sh <pkg> <version> [--push]}
ver=${2:?usage: publish.sh <pkg> <version> [--push]}
push=${3:-}

src_dir=$(cd "$(dirname "$0")/$pkg" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cp "$src_dir/PKGBUILD" "$work/"
cd "$work"

sed -i -E "s/^pkgver=.*/pkgver=$ver/; s/^pkgrel=.*/pkgrel=1/" PKGBUILD
updpkgsums
makepkg --printsrcinfo > .SRCINFO

# Build and run check() so a broken PKGBUILD never reaches the AUR.
makepkg -f --noconfirm --nocolor

if command -v namcap >/dev/null; then
  { namcap PKGBUILD; namcap ./*.pkg.tar.*; } | tee namcap.log
  if grep -q ' E: ' namcap.log; then
    echo "namcap reported errors, not publishing $pkg" >&2
    exit 1
  fi
fi

if [ "$push" != "--push" ]; then
  echo "validated $pkg $ver (no --push, not publishing)"
  exit 0
fi

: "${GIT_SSH_COMMAND:=ssh -i $HOME/.ssh/aur -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=$HOME/.ssh/known_hosts}"
export GIT_SSH_COMMAND

git clone "${AUR_GIT_BASE:-ssh://aur@aur.archlinux.org}/$pkg.git" repo
cp PKGBUILD .SRCINFO repo/
cd repo
git add PKGBUILD .SRCINFO
if git diff --cached --quiet; then
  echo "$pkg $ver already published, nothing to do"
  exit 0
fi
git commit -m "Update to $ver"
git push origin HEAD:master
