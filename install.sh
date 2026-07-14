#!/usr/bin/env bash
# One-line installer for bluebird (fallback for platforms without Homebrew).
# Usage: curl -fsSL https://raw.githubusercontent.com/x0c/bluebird-cli/main/install.sh | bash
set -euo pipefail

REPO="x0c/bluebird-cli"
INSTALL_DIR="${BLUEBIRD_INSTALL_DIR:-$HOME/.local/bin}"

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "Error: '$1' is required but not found" >&2; exit 1; }
}
need curl
need tar
need uname
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo "Error: need either 'sha256sum' or 'shasum' to verify the download" >&2
  exit 1
fi

os="$(uname -s)"
case "$os" in
  Darwin) goos="darwin" ;;
  Linux) goos="linux" ;;
  *) echo "Error: unsupported OS: $os" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "Error: unsupported architecture: $arch" >&2; exit 1 ;;
esac

VERSION="${BLUEBIRD_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
fi
if [ -z "$VERSION" ]; then
  echo "Error: could not determine latest version; set BLUEBIRD_VERSION explicitly" >&2
  exit 1
fi

archive="bluebird_${goos}_${goarch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${VERSION}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

echo "Installing bluebird ${VERSION} (${goos}/${goarch}) ..."
curl -fsSL -o "$workdir/$archive" "${base_url}/${archive}"
curl -fsSL -o "$workdir/checksums.txt" "${base_url}/checksums.txt"

(
  cd "$workdir"
  if command -v sha256sum >/dev/null 2>&1; then
    grep " ${archive}\$" checksums.txt | sha256sum -c -
  else
    expected=$(grep " ${archive}\$" checksums.txt | awk '{print $1}')
    actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    [ "$expected" = "$actual" ] || { echo "Error: checksum mismatch for $archive" >&2; exit 1; }
  fi
)

tar -xzf "$workdir/$archive" -C "$workdir" bluebird
mkdir -p "$INSTALL_DIR"
install -m 755 "$workdir/bluebird" "$INSTALL_DIR/bluebird"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    echo "Installed. Run 'bluebird version' to verify."
    ;;
  *)
    echo ""
    echo "Installed to ${INSTALL_DIR}, which is not in your PATH."
    echo "Add this to your shell profile (e.g. ~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac
