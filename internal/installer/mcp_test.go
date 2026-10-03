package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/templates"
)

// legacyMCP is what releases before v0.4.5 installed: unpinned packages, one of
// which does not exist on npm. Sync keeps an existing file, so it stays.
const legacyMCP = `{"mcpServers":{
  "chrome-devtools":{"command":"npx","args":["-y","@modelcontextprotocol/server-chrome-devtools"]},
  "playwright":{"command":"npx","args":["-y","@executeautomation/playwright-mcp-server"]}}}`

func writeMCP(t *testing.T, info *platform.Info, content string) {
	t.Helper()
	p := MCPPath(info)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckMCP(t *testing.T) {
	builtin, _ := templates.FS.ReadFile(mcpTemplate)
	for _, tc := range []struct {
		name, content          string
		exists, differs, inval bool
		unpinned               int
	}{
		{name: "missing"},
		{name: "builtin", content: string(builtin), exists: true},
		{name: "builtin reformatted", content: strings.Join(strings.Fields(string(builtin)), ""), exists: true},
		{name: "legacy", content: legacyMCP, exists: true, differs: true, unpinned: 2},
		{name: "pinned but custom", content: `{"mcpServers":{"x":{"command":"npx","args":["-y","x@1.2.3"]}}}`, exists: true, differs: true},
		{name: "invalid", content: `{`, exists: true, differs: true, inval: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &platform.Info{GeminiDir: t.TempDir()}
			if tc.content != "" {
				writeMCP(t, info, tc.content)
			}
			st := CheckMCP(info)
			if st.Exists != tc.exists || st.Differs != tc.differs || st.Invalid != tc.inval || len(st.Unpinned) != tc.unpinned {
				t.Fatalf("got %+v", st)
			}
			if st.NeedsUpdate() != (tc.differs || tc.unpinned > 0) {
				t.Fatalf("NeedsUpdate=%v for %+v", st.NeedsUpdate(), st)
			}
		})
	}
}

func TestCheckMCP_ReportsUnpinnedPackages(t *testing.T) {
	info := &platform.Info{GeminiDir: t.TempDir()}
	writeMCP(t, info, legacyMCP)
	got := strings.Join(CheckMCP(info).Unpinned, "|")
	want := "chrome-devtools: @modelcontextprotocol/server-chrome-devtools|playwright: @executeautomation/playwright-mcp-server"
	if got != want {
		t.Fatalf("Unpinned = %q, want %q", got, want)
	}
}

func TestUpdateMCP_BacksUpAndRewrites(t *testing.T) {
	info := &platform.Info{GeminiDir: t.TempDir()}
	writeMCP(t, info, legacyMCP)

	// Plain sync keeps the legacy file (existing behaviour, by design).
	if _, err := Sync(info); err != nil {
		t.Fatal(err)
	}
	if !CheckMCP(info).NeedsUpdate() {
		t.Fatal("legacy manifest should still need an update after plain sync")
	}

	now := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	backup, changed, err := UpdateMCP(info, now)
	if err != nil || !changed {
		t.Fatalf("UpdateMCP: changed=%v err=%v", changed, err)
	}
	if want := MCPPath(info) + ".bak-20261003T070000Z"; backup != want {
		t.Fatalf("backup = %q, want %q", backup, want)
	}
	if old, _ := os.ReadFile(backup); string(old) != legacyMCP {
		t.Fatalf("backup content changed: %s", old)
	}
	if fi, _ := os.Stat(backup); fi.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, want 0600", fi.Mode().Perm())
	}
	builtin, _ := templates.FS.ReadFile(mcpTemplate)
	if cur, _ := os.ReadFile(MCPPath(info)); string(cur) != string(builtin) {
		t.Fatalf("manifest not rewritten: %s", cur)
	}
	if st := CheckMCP(info); st.NeedsUpdate() {
		t.Fatalf("still needs update after rewrite: %+v", st)
	}

	// Second run is a no-op: no new backup, no write.
	backup, changed, err = UpdateMCP(info, now.Add(time.Hour))
	if err != nil || changed || backup != "" {
		t.Fatalf("second UpdateMCP: backup=%q changed=%v err=%v", backup, changed, err)
	}
}

func TestUpdateMCP_InstallsWhenMissing(t *testing.T) {
	info := &platform.Info{GeminiDir: t.TempDir()}
	backup, changed, err := UpdateMCP(info, time.Now())
	if err != nil || !changed || backup != "" {
		t.Fatalf("backup=%q changed=%v err=%v", backup, changed, err)
	}
	if !CheckMCP(info).Exists {
		t.Fatal("manifest not installed")
	}
}
