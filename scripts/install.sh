#!/bin/sh
# install.sh — installs the latest podstack release for macOS or Linux.
#
# Usage:
#   curl -fsSL https://podstack.ai/install.sh | sh
#   # or
#   sh install.sh
#
# Environment overrides:
#   PODSTACK_VERSION   pin a specific tag, e.g. "v1.2.3" (default: latest release)
#   PODSTACK_INSTALL_DIR  install location (default: /usr/local/bin or ~/.local/bin)

set -eu

REPO="Podstack-ai/podstack-cli"
GH_API="https://api.github.com/repos/${REPO}"
GH_DL="https://github.com/${REPO}/releases/download"

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
note() { printf '==> %s\n' "$*"; }

# --- detect OS and architecture ----------------------------------------------
uname_s=$(uname -s 2>/dev/null || echo unknown)
uname_m=$(uname -m 2>/dev/null || echo unknown)

case "$uname_s" in
  Darwin)  OS=darwin ;;
  Linux)   OS=linux ;;
  *)       err "unsupported OS: $uname_s (only macOS and Linux supported)" ;;
esac

case "$uname_m" in
  x86_64|amd64)    ARCH=amd64 ;;
  arm64|aarch64)   ARCH=arm64 ;;
  *)               err "unsupported architecture: $uname_m" ;;
esac

# --- determine version -------------------------------------------------------
VERSION=${PODSTACK_VERSION:-}
if [ -z "$VERSION" ]; then
  note "Fetching latest release tag"
  VERSION=$(curl -fsSL "${GH_API}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || err "could not determine latest release tag"
fi
note "Installing podstack ${VERSION} for ${OS}/${ARCH}"

# --- download tarball + checksums --------------------------------------------
TARBALL="podstack_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d 2>/dev/null || mktemp -d -t podstack)
trap 'rm -rf "$TMP"' EXIT

note "Downloading ${TARBALL}"
curl -fsSL -o "${TMP}/${TARBALL}" "${GH_DL}/${VERSION}/${TARBALL}" \
  || err "download failed: ${GH_DL}/${VERSION}/${TARBALL}"

note "Downloading checksums.txt"
curl -fsSL -o "${TMP}/checksums.txt" "${GH_DL}/${VERSION}/checksums.txt" \
  || err "checksum file download failed"

# --- verify sha256 -----------------------------------------------------------
note "Verifying sha256"
EXPECTED=$(grep " ${TARBALL}\$" "${TMP}/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || err "no checksum entry for ${TARBALL}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP}/${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP}/${TARBALL}" | awk '{print $1}')
else
  err "neither sha256sum nor shasum found; cannot verify integrity"
fi

[ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch: expected $EXPECTED got $ACTUAL"

# --- extract -----------------------------------------------------------------
note "Extracting"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"
[ -f "${TMP}/podstack" ] || err "tarball did not contain a 'podstack' binary"
chmod +x "${TMP}/podstack"

# --- choose install dir ------------------------------------------------------
INSTALL_DIR=${PODSTACK_INSTALL_DIR:-}
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    note "Installing to ${INSTALL_DIR} (not on /usr/local/bin — add it to PATH if needed)"
  fi
fi

mv "${TMP}/podstack" "${INSTALL_DIR}/podstack"
note "Installed: ${INSTALL_DIR}/podstack"

# --- confirm -----------------------------------------------------------------
"${INSTALL_DIR}/podstack" version
