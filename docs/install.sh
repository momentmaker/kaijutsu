#!/bin/sh
# kaijutsu installer — fetches the latest jutsu binary for your OS/arch
# from the momentmaker/kaijutsu GitHub release and drops it into your
# preferred bin directory.
#
# Usage:
#   curl -fsSL https://kaijutsu.dev/install.sh | sh
#
# Override the install dir:
#   curl -fsSL https://kaijutsu.dev/install.sh | KAIJUTSU_INSTALL_DIR=/usr/local/bin sh

set -eu

REPO="momentmaker/kaijutsu"
INSTALL_DIR="${KAIJUTSU_INSTALL_DIR:-${HOME}/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *) echo "kaijutsu: unsupported OS: $OS" >&2; exit 1 ;;
esac

RAW_ARCH=$(uname -m)
case "$RAW_ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "kaijutsu: unsupported arch: $RAW_ARCH" >&2; exit 1 ;;
esac

# Resolve the latest release tag.
LATEST_JSON=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
TAG=$(echo "$LATEST_JSON" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
if [ -z "$TAG" ]; then
  echo "kaijutsu: could not resolve latest release tag from ${REPO}" >&2
  exit 1
fi
VERSION=${TAG#v}

ARCHIVE="jutsu_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "kaijutsu: downloading ${ARCHIVE}"
curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

mkdir -p "$INSTALL_DIR"
mv "${TMP}/jutsu" "${INSTALL_DIR}/jutsu"
chmod +x "${INSTALL_DIR}/jutsu"

echo "kaijutsu: installed jutsu ${VERSION} to ${INSTALL_DIR}/jutsu"
case ":${PATH}:" in
  *:"$INSTALL_DIR":*) ;;
  *) echo "kaijutsu: add ${INSTALL_DIR} to your PATH to run jutsu";;
esac
