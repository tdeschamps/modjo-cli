#!/bin/sh
# Modjo CLI installer. Downloads the latest signed release binary for the host
# platform from GitHub Releases and installs it to a bin directory on PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/tdeschamps/modjo-cli/main/install.sh | sh
#
# Override the install dir with MODJO_INSTALL_DIR, or the version with
# MODJO_VERSION (defaults to the latest release).
set -eu

REPO="tdeschamps/modjo-cli"
INSTALL_DIR="${MODJO_INSTALL_DIR:-/usr/local/bin}"
VERSION="${MODJO_VERSION:-latest}"

err() { echo "install: $*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac
case "$os" in
  linux|darwin) ;;
  *) err "unsupported OS: $os (use scoop/winget on Windows)" ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | cut -d'"' -f4)
  [ -n "$VERSION" ] || err "could not resolve latest version"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

asset="modjo_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
echo "Downloading ${url}"
curl -fsSL "$url" -o "$tmp/$asset" || err "download failed"
tar -xzf "$tmp/$asset" -C "$tmp"

if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp/modjo" "$INSTALL_DIR/modjo"
else
  echo "Elevating to install into $INSTALL_DIR"
  sudo mv "$tmp/modjo" "$INSTALL_DIR/modjo"
fi
chmod +x "$INSTALL_DIR/modjo" 2>/dev/null || true

echo "Installed: $("$INSTALL_DIR/modjo" version 2>/dev/null || echo "$INSTALL_DIR/modjo")"
