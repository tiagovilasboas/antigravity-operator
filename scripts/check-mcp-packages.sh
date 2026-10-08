#!/usr/bin/env bash
# Fails when a templated MCP server names an npm package@version that the
# registry does not serve, that is deprecated, or that runs install-time
# lifecycle scripts (preinstall/install/postinstall). Needs node and npm.
# Run: bash scripts/check-mcp-packages.sh
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
fail=0

pkgs="$(node -e '
  const fs = require("fs"), path = require("path");
  const dir = path.join(process.argv[1], "templates", "mcps");
  for (const f of fs.readdirSync(dir).filter((f) => f.endsWith(".json"))) {
    const servers = JSON.parse(fs.readFileSync(path.join(dir, f), "utf8")).mcpServers || {};
    for (const s of Object.values(servers)) {
      if (s.command !== "npx") continue;
      const pkg = (s.args || []).find((a) => !a.startsWith("-"));
      if (pkg) console.log(pkg);
    }
  }
' "$DIR")"

if [ -z "$pkgs" ]; then
  echo "no npx packages found in templates/mcps"
  exit 1
fi

for spec in $pkgs; do
  name="${spec%@*}"
  want="${spec##*@}"
  if [ -z "$name" ] || [ "$name" = "$spec" ]; then
    echo "FAIL - $spec is not pinned as name@version"
    fail=1
    continue
  fi
  if ! meta="$(npm view "$name@$want" version deprecated scripts --json 2>/dev/null)" || [ -z "$meta" ]; then
    echo "FAIL - $name@$want not found on npm"
    fail=1
    continue
  fi
  # shellcheck disable=SC2016 # ${...} below is a JavaScript template literal
  if problems="$(node -e '
    const want = process.argv[1];
    let m = JSON.parse(process.argv[2]);
    if (Array.isArray(m)) m = m[m.length - 1];
    if (typeof m === "string") m = { version: m };
    const out = [];
    if (m.version !== want) out.push(`registry returned version ${m.version}`);
    if (m.deprecated) out.push(`deprecated: ${m.deprecated}`);
    for (const s of ["preinstall", "install", "postinstall"]) {
      if (m.scripts && m.scripts[s]) out.push(`runs a ${s} script: ${m.scripts[s]}`);
    }
    console.log(out.join("; "));
  ' "$want" "$meta")" && [ -z "$problems" ]; then
    echo "ok   - $name@$want exists, not deprecated, no install scripts"
  else
    echo "FAIL - $name@$want: ${problems:-could not parse npm metadata}"
    fail=1
  fi
done

exit "$fail"
