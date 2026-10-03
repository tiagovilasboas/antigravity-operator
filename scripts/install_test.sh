#!/usr/bin/env bash
# Tests for the checksum verification in scripts/install.sh.
# Run: bash scripts/install_test.sh
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=SCRIPTDIR/install.sh
AGYO_INSTALL_LIB=1 source "$DIR/install.sh" >/dev/null

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

printf 'agyo binary\n' > "$TMP/agyo_linux_amd64.tar.gz"
good="$(sha256_of "$TMP/agyo_linux_amd64.tar.gz")"
fail=0

check() {
  local want="$1" desc="$2"; shift 2
  if "$@" 2>/dev/null; then got=pass; else got=fail; fi
  if [ "$got" = "$want" ]; then
    echo "ok   - $desc"
  else
    echo "FAIL - $desc (want $want, got $got)"
    fail=1
  fi
}

printf '%s  agyo_linux_amd64.tar.gz\n%s  other.zip\n' "$good" "$good" > "$TMP/sums"
check pass "matching digest" verify_checksum "$TMP/agyo_linux_amd64.tar.gz" "$TMP/sums" agyo_linux_amd64.tar.gz

printf '%s *agyo_linux_amd64.tar.gz\n' "$good" > "$TMP/sums-bin"
check pass "binary-mode entry (*name)" verify_checksum "$TMP/agyo_linux_amd64.tar.gz" "$TMP/sums-bin" agyo_linux_amd64.tar.gz

printf '%s  agyo_linux_amd64.tar.gz\n' "$(printf '0%.0s' $(seq 64))" > "$TMP/sums-bad"
check fail "mismatched digest" verify_checksum "$TMP/agyo_linux_amd64.tar.gz" "$TMP/sums-bad" agyo_linux_amd64.tar.gz

printf '%s  agyo_darwin_arm64.tar.gz\n' "$good" > "$TMP/sums-missing"
check fail "no entry for the file" verify_checksum "$TMP/agyo_linux_amd64.tar.gz" "$TMP/sums-missing" agyo_linux_amd64.tar.gz

printf '%s  xagyo_linux_amd64.tar.gz\n' "$good" > "$TMP/sums-prefix"
check fail "similar name is not a match" verify_checksum "$TMP/agyo_linux_amd64.tar.gz" "$TMP/sums-prefix" agyo_linux_amd64.tar.gz

exit "$fail"
