#!/usr/bin/env bash
# Fails when a templated MCP server names an npm package@version that the
# registry does not serve. Needs node/npm. Run: bash scripts/check-mcp-packages.sh
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
  if got="$(npm view "$name@$want" version 2>/dev/null)" && [ "$got" = "$want" ]; then
    echo "ok   - $name@$want exists on npm"
  else
    echo "FAIL - $name@$want not found on npm"
    fail=1
  fi
done

exit "$fail"
