#!/usr/bin/env bash
set -euo pipefail

# Antigravity Operator (agyo) — Universal One-Line Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash

REPO="tiagovilasboas/antigravity-operator"
BINARY="agyo"
ALIAS="antigravity-operator"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "❌ Unsupported architecture: $ARCH" && exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "❌ Unsupported OS: $OS" && exit 1 ;;
esac

echo "🚀 Installing Antigravity Operator (agyo) for $OS/$ARCH..."

# sha256_of FILE prints the SHA-256 hex digest of FILE.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# verify_checksum FILE CHECKSUMS NAME succeeds only when CHECKSUMS has a line
# for NAME and its digest matches FILE.
verify_checksum() {
  local file="$1" sums="$2" name="$3" expected actual
  expected="$(awk -v n="$name" '$2 == n || $2 == "*"n {print $1; exit}' "$sums")"
  if [ -z "$expected" ]; then
    echo "❌ No checksum for $name in checksums.txt" >&2
    return 1
  fi
  actual="$(sha256_of "$file")"
  if [ "$expected" != "$actual" ]; then
    echo "❌ Checksum mismatch for $name (expected $expected, got $actual)" >&2
    return 1
  fi
}

# latest_tag_from_url URL prints the tag from a GitHub ".../releases/tag/<tag>"
# URL (where /releases/latest redirects), or nothing for any other URL.
latest_tag_from_url() {
  case "$1" in
    */releases/tag/*) printf '%s\n' "${1##*/releases/tag/}" ;;
  esac
}

# Tests source this file with AGYO_INSTALL_LIB=1 to get the functions only.
if [ "${AGYO_INSTALL_LIB:-}" = "1" ]; then
  # shellcheck disable=SC2317 # exit is the fallback when executed, not sourced
  return 0 2>/dev/null || exit 0
fi

# Determine installation directory
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# Resolve the latest release tag from the github.com redirect, not the REST API:
# the anonymous API limit (60 requests/hour per IP) returns 403 behind shared
# NAT or CI runners, which silently skipped the verified binary.
LATEST_URL="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" || true)"
LATEST_TAG="$(latest_tag_from_url "$LATEST_URL")"

DOWNLOADED=0
if [ -n "$LATEST_TAG" ]; then
  TARBALL="agyo_${OS}_${ARCH}.tar.gz"
  BASE_URL="https://github.com/$REPO/releases/download/$LATEST_TAG"

  TMP_DIR="$(mktemp -d)"
  if curl -fsSL "$BASE_URL/$TARBALL" -o "$TMP_DIR/$TARBALL"; then
    if ! curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP_DIR/checksums.txt"; then
      echo "⚠️  Release $LATEST_TAG has no checksums.txt; refusing to install an unverified binary."
      rm -rf "$TMP_DIR"
      TMP_DIR=""
    elif ! verify_checksum "$TMP_DIR/$TARBALL" "$TMP_DIR/checksums.txt" "$TARBALL"; then
      rm -rf "$TMP_DIR"
      exit 1
    else
      echo "🔒 Verified SHA-256 of $TARBALL against checksums.txt"
    fi
  fi
  if [ -n "$TMP_DIR" ] && [ -f "$TMP_DIR/$TARBALL" ] && tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR" 2>/dev/null; then
    EXTRACTED_BIN="$(find "$TMP_DIR" -type f \( -name "agyo" -o -name "agyo_*" \) ! -name "*.tar.gz" ! -name "*.zip" | head -n 1)"
    if [ -n "$EXTRACTED_BIN" ]; then
      mv "$EXTRACTED_BIN" "$INSTALL_DIR/$BINARY"
      chmod +x "$INSTALL_DIR/$BINARY"
      ln -sf "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/$ALIAS"
      rm -rf "$TMP_DIR"
      DOWNLOADED=1
    fi
  fi
fi

# Fallback: If no release tarball or release is pending, compile from source if Go is installed
if [ "$DOWNLOADED" -eq 0 ]; then
  if command -v go >/dev/null 2>&1; then
    echo "📦 Release binary pending; compiling static binary from source via Go..."
    TMP_SRC="$(mktemp -d)"
    git clone --depth 1 "https://github.com/$REPO.git" "$TMP_SRC"
    (cd "$TMP_SRC" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$INSTALL_DIR/$BINARY" ./cmd/agyo)
    ln -sf "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/$ALIAS"
    rm -rf "$TMP_SRC"
    DOWNLOADED=1
  else
    echo "❌ Pre-built binary not found for $OS/$ARCH and Go toolchain is missing."
    echo "   Please install Go (https://go.dev) or download from https://github.com/$REPO/releases"
    exit 1
  fi
fi

echo "✅ Successfully installed 'agyo' and 'antigravity-operator' into $INSTALL_DIR!"
echo ""
echo "Verify installation:"
echo "  agyo version"
echo "  agyo doctor"
