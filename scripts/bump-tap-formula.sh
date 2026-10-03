#!/usr/bin/env bash
# Opens a pull request on the Homebrew tap that bumps Formula/agyo.rb to a tag.
# Used by .github/workflows/release.yml after a release is published.
#
# Env: TAG (e.g. v0.4.6), GH_TOKEN (token with contents:write and
# pull-requests:write on TAP_REPO), optional SOURCE_REPO, TAP_REPO, TAP_BASE.
# The commit is created with the createCommitOnBranch GraphQL mutation, so it
# is signed by GitHub and attributed to the token owner.
set -euo pipefail

# bump_formula FILE URL SHA256 rewrites the url and sha256 lines of a formula.
bump_formula() {
  local file="$1" url="$2" sha="$3"
  case "$sha" in
    *[!0-9a-f]* | "") echo "invalid sha256: $sha" >&2; return 1 ;;
  esac
  [ "${#sha}" -eq 64 ] || { echo "invalid sha256 length: $sha" >&2; return 1; }
  if ! grep -q '^  url "' "$file" || ! grep -q '^  sha256 "' "$file"; then
    echo "formula has no top-level url/sha256 lines: $file" >&2
    return 1
  fi
  sed -i.bak -e "s|^  url \".*\"|  url \"${url}\"|" -e "s|^  sha256 \".*\"|  sha256 \"${sha}\"|" "$file"
  rm -f "$file.bak"
}

# Tests source this file with BUMP_TAP_LIB=1 to get the functions only.
if [ "${BUMP_TAP_LIB:-}" = "1" ]; then
  # shellcheck disable=SC2317 # exit is the fallback when executed, not sourced
  return 0 2>/dev/null || exit 0
fi

: "${TAG:?TAG is required}"
: "${GH_TOKEN:?GH_TOKEN is required}"
SOURCE_REPO="${SOURCE_REPO:-tiagovilasboas/antigravity-operator}"
TAP_REPO="${TAP_REPO:-tiagovilasboas/homebrew-tap}"
TAP_BASE="${TAP_BASE:-main}"
FORMULA="Formula/agyo.rb"
BRANCH="bump-agyo-${TAG}"
URL="https://github.com/${SOURCE_REPO}/archive/refs/tags/${TAG}.tar.gz"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

curl -fsSL "$URL" -o "$WORK/src.tar.gz"
SHA="$(sha256sum "$WORK/src.tar.gz" | awk '{print $1}')"
echo "source tarball ${URL} sha256 ${SHA}"

gh api "repos/${TAP_REPO}/contents/${FORMULA}?ref=${TAP_BASE}" -H 'Accept: application/vnd.github.raw' > "$WORK/agyo.rb"
cp "$WORK/agyo.rb" "$WORK/agyo.rb.orig"
bump_formula "$WORK/agyo.rb" "$URL" "$SHA"
if cmp -s "$WORK/agyo.rb" "$WORK/agyo.rb.orig"; then
  echo "tap formula already at ${TAG}; nothing to do"
  exit 0
fi

if gh api "repos/${TAP_REPO}/branches/${BRANCH}" >/dev/null 2>&1; then
  echo "branch ${BRANCH} already exists on ${TAP_REPO}; not opening another PR"
  exit 0
fi
BASE_SHA="$(gh api "repos/${TAP_REPO}/git/ref/heads/${TAP_BASE}" --jq .object.sha)"
gh api -X POST "repos/${TAP_REPO}/git/refs" -f ref="refs/heads/${BRANCH}" -f sha="$BASE_SHA" >/dev/null

CONTENT="$(base64 < "$WORK/agyo.rb" | tr -d '\n')"
# shellcheck disable=SC2016 # GraphQL variables, not shell expansions
QUERY='mutation($input: CreateCommitOnBranchInput!) { createCommitOnBranch(input: $input) { commit { oid } } }'
jq -n --arg q "$QUERY" --arg repo "$TAP_REPO" --arg branch "$BRANCH" --arg head "$BASE_SHA" \
  --arg msg "chore(formula): bump agyo to ${TAG}" --arg path "$FORMULA" --arg content "$CONTENT" \
  '{query: $q, variables: {input: {branch: {repositoryNameWithOwner: $repo, branchName: $branch},
    expectedHeadOid: $head, message: {headline: $msg},
    fileChanges: {additions: [{path: $path, contents: $content}]}}}}' > "$WORK/req.json"
gh api graphql --input "$WORK/req.json" --jq .data.createCommitOnBranch.commit.oid

gh pr create -R "$TAP_REPO" --base "$TAP_BASE" --head "$BRANCH" \
  --title "chore(formula): bump agyo to ${TAG}" \
  --body "Automated bump from the ${SOURCE_REPO} release workflow.

- url: ${URL}
- sha256: \`${SHA}\` (computed from the tag source tarball)

Release: https://github.com/${SOURCE_REPO}/releases/tag/${TAG}"
