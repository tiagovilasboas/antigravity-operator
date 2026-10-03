#!/usr/bin/env bash
# Tests for bump_formula in scripts/bump-tap-formula.sh.
# Run: bash scripts/bump-tap-formula_test.sh
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=SCRIPTDIR/bump-tap-formula.sh
BUMP_TAP_LIB=1 source "$DIR/bump-tap-formula.sh"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
fail=0
# check WANT DESC CMD...: WANT is pass or fail.
check() {
  local want="$1" desc="$2"; shift 2
  if "$@" 2>/dev/null; then got=pass; else got=fail; fi
  if [ "$got" = "$want" ]; then echo "ok   - $desc"; else echo "FAIL - $desc (want $want, got $got)"; fail=1; fi
}
# shellcheck disable=SC2329 # invoked indirectly through check
same_lines() { [ "$(wc -l < "$1")" -eq "$(wc -l < "$2")" ]; }

SHA="$(printf 'a%.0s' $(seq 64))"
URL="https://github.com/o/r/archive/refs/tags/v9.9.9.tar.gz"

F="$DIR/../Formula/agyo.rb"
cp "$F" "$TMP/agyo.rb"
check pass "valid bump succeeds" bump_formula "$TMP/agyo.rb" "$URL" "$SHA"
check pass "url rewritten" grep -qx "  url \"$URL\"" "$TMP/agyo.rb"
check pass "sha256 rewritten" grep -qx "  sha256 \"$SHA\"" "$TMP/agyo.rb"
check pass "head url untouched" grep -q 'head "https://github.com/tiagovilasboas/antigravity-operator.git"' "$TMP/agyo.rb"
check pass "version ldflag kept" grep -q -- '-X main.Version=#{version}' "$TMP/agyo.rb"
check pass "no lines added or removed" same_lines "$TMP/agyo.rb" "$F"

cp "$F" "$TMP/bad.rb"
check fail "rejects invalid sha256" bump_formula "$TMP/bad.rb" "$URL" "not-a-sha"
check fail "rejects short sha256" bump_formula "$TMP/bad.rb" "$URL" "abc"
check pass "rejected input leaves file unchanged" cmp -s "$TMP/bad.rb" "$F"

printf 'class X < Formula\nend\n' > "$TMP/empty.rb"
check fail "rejects formula without url/sha256" bump_formula "$TMP/empty.rb" "$URL" "$SHA"

exit "$fail"
