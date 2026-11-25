#!/bin/bash
# commit-msg hook: remove AI-agent attribution from a commit message.
#
# Works for every harness because it matches the text, not the session: Claude
# Code, Cursor, Codex, Copilot, Gemini, Kiro and friends all end up as a line in
# the message file git hands to this hook. One grep and one awk over a few lines,
# so it adds a few milliseconds. It strips instead of rejecting: a rejection makes
# the agent retry the commit and burn tokens.
#
# Usage: commit-msg <message-file>   (git passes the path as $1)
#        printf 'msg' | strip-agent-attribution.sh   (prints the cleaned message)
#
# Author/committer identity cannot be rewritten from a commit-msg hook, so an
# agent identity still fails the commit (exit 1).

AGENT_EMAIL='noreply@anthropic\.com|cursoragent@cursor\.com|noreply@openai\.com|copilot@github\.com|[0-9]+\+copilot@users\.noreply\.github\.com'
AGENT_NAME='(claude|cursor( agent)?|codex|(github )?copilot|gemini|antigravity|kiro|grok)([ -][a-z0-9.]+)*'
TOOL='(claude( code)?|cursor( agent| cli)?|codex( cli)?|(github )?copilot|gemini( cli)?|antigravity|kiro( cli)?)'

PATTERN="^[[:space:]]*(co-authored-by:.*<(${AGENT_EMAIL})>[[:space:]]*|co-authored-by:[[:space:]]*${AGENT_NAME}[[:space:]]*<[^>]*noreply[^>]*>[[:space:]]*|made-with:[[:space:]]*(cursor|claude|codex|copilot)[a-z .-]*|claude-session:.*|([^[:alnum:][:space:]]+[[:space:]]*)?generated (with|by) \\[?${TOOL}\\]?(\\(https?://[^)]*\\))?[[:space:]]*)$"

clean() {
  # Drop agent lines, squeeze the blank runs they leave behind, trim trailing blanks.
  grep -viE "$PATTERN" | awk '
    /^[[:space:]]*$/ { blank++; next }
    { if (blank && printed) print ""; blank = 0; print; printed = 1 }'
}

if [ -n "$1" ] && [ -f "$1" ]; then
  tmp=$(mktemp) || exit 0
  clean < "$1" > "$tmp"
  if [ -s "$tmp" ] && cp "$tmp" "$1.new" && mv "$1.new" "$1"; then :; else rm -f "$1.new"; fi
  rm -f "$tmp"
else
  clean
  exit 0
fi

if git rev-parse --git-dir >/dev/null 2>&1; then
  for who in GIT_AUTHOR_IDENT GIT_COMMITTER_IDENT; do
    ident=$(git var "$who" 2>/dev/null)
    if printf '%s' "$ident" | grep -qiE "$AGENT_EMAIL"; then
      {
        echo "GUARD RAIL: ${who%_IDENT} is an agent identity: $ident"
        echo "Use your own git identity (git config user.name / user.email)."
      } >&2
      exit 1
    fi
  done
fi
exit 0
