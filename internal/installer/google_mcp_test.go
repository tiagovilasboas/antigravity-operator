package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func readMCPServers(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest %s: %v", path, err)
	}
	var cfg struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal manifest %s: %v", path, err)
	}
	return cfg.MCPServers
}

func TestInstallCalendarMCP_FreshAndStatusAndUninstall(t *testing.T) {
	home := t.TempDir()
	info := &platform.Info{HomeDir: home}

	stBefore, err := CalendarMCPStatus(info)
	if err != nil {
		t.Fatalf("CalendarMCPStatus before install: %v", err)
	}
	if stBefore.Installed || stBefore.SchemasCount != 0 {
		t.Fatalf("expected uninstalled before install, got %+v", stBefore)
	}

	res, err := InstallCalendarMCP(info)
	if err != nil {
		t.Fatalf("InstallCalendarMCP: %v", err)
	}
	if !res.Installed || res.Name != GoogleCalendarMCPServerName || res.SchemasCount != len(calendarToolSchemas) {
		t.Fatalf("unexpected install result: %+v", res)
	}

	defaultPath := filepath.Join(home, ".gemini", "antigravity", "mcp", "default-servers.json")
	userPath := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	schemaDir := filepath.Join(home, ".gemini", "antigravity", "mcp", GoogleCalendarMCPServerName)

	if _, ok := readMCPServers(t, defaultPath)[GoogleCalendarMCPServerName]; !ok {
		t.Fatalf("missing %s in %s", GoogleCalendarMCPServerName, defaultPath)
	}
	if _, ok := readMCPServers(t, userPath)[GoogleCalendarMCPServerName]; !ok {
		t.Fatalf("missing %s in %s", GoogleCalendarMCPServerName, userPath)
	}
	for filename := range calendarToolSchemas {
		if _, err := os.Stat(filepath.Join(schemaDir, filename)); err != nil {
			t.Fatalf("expected schema file %s: %v", filename, err)
		}
	}

	// Non-json files and subdirectories should not inflate SchemasCount.
	_ = os.WriteFile(filepath.Join(schemaDir, "README.txt"), []byte("ignore"), 0644)
	_ = os.MkdirAll(filepath.Join(schemaDir, "nested.json"), 0755)

	stAfter, err := CalendarMCPStatus(info)
	if err != nil {
		t.Fatalf("CalendarMCPStatus after install: %v", err)
	}
	if !stAfter.Installed || stAfter.SchemasCount != len(calendarToolSchemas) {
		t.Fatalf("expected installed with %d schemas, got %+v", len(calendarToolSchemas), stAfter)
	}

	if err := UninstallCalendarMCP(info); err != nil {
		t.Fatalf("UninstallCalendarMCP: %v", err)
	}
	// Idempotent second uninstall.
	if err := UninstallCalendarMCP(info); err != nil {
		t.Fatalf("second UninstallCalendarMCP: %v", err)
	}

	if _, err := os.Stat(schemaDir); !os.IsNotExist(err) {
		t.Fatalf("expected schema dir %s removed, stat err=%v", schemaDir, err)
	}
	stUninstalled, err := CalendarMCPStatus(info)
	if err != nil {
		t.Fatalf("CalendarMCPStatus after uninstall: %v", err)
	}
	if stUninstalled.Installed || stUninstalled.SchemasCount != 0 {
		t.Fatalf("expected uninstalled after uninstall, got %+v", stUninstalled)
	}
}

func TestInstallAndUninstallCalendarMCP_PreservesExistingServers(t *testing.T) {
	home := t.TempDir()
	info := &platform.Info{
		HomeDir:   home,
		GeminiDir: filepath.Join(home, ".gemini", "antigravity"),
	}

	defaultPath := filepath.Join(info.GeminiDir, "mcp", "default-servers.json")
	userPath := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	seed := []byte(`{"mcpServers":{"github":{"command":"npx","args":["-y","@modelcontextprotocol/server-github"]}}}`)

	for _, p := range []string{defaultPath, userPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, seed, 0644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := InstallCalendarMCP(info); err != nil {
		t.Fatalf("InstallCalendarMCP: %v", err)
	}
	for _, p := range []string{defaultPath, userPath} {
		servers := readMCPServers(t, p)
		if _, ok := servers["github"]; !ok {
			t.Fatalf("expected pre-existing server 'github' preserved in %s", p)
		}
		if _, ok := servers[GoogleCalendarMCPServerName]; !ok {
			t.Fatalf("expected %s registered in %s", GoogleCalendarMCPServerName, p)
		}
	}

	if err := UninstallCalendarMCP(info); err != nil {
		t.Fatalf("UninstallCalendarMCP: %v", err)
	}
	for _, p := range []string{defaultPath, userPath} {
		servers := readMCPServers(t, p)
		if _, ok := servers["github"]; !ok {
			t.Fatalf("expected sibling server 'github' preserved after uninstall in %s", p)
		}
		if _, ok := servers[GoogleCalendarMCPServerName]; ok {
			t.Fatalf("expected %s removed from %s", GoogleCalendarMCPServerName, p)
		}
	}
}

func TestCalendarMCP_MalformedJSONAndSchemaDirError(t *testing.T) {
	home := t.TempDir()
	info := &platform.Info{HomeDir: home}

	defaultPath := filepath.Join(home, ".gemini", "antigravity", "mcp", "default-servers.json")
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0755); err != nil {
		t.Fatal(err)
	}
	badJSON := []byte(`{"mcpServers": {broken`)
	if err := os.WriteFile(defaultPath, badJSON, 0644); err != nil {
		t.Fatal(err)
	}

	if isServerInManifest(defaultPath, GoogleCalendarMCPServerName) {
		t.Fatal("expected isServerInManifest false for malformed JSON")
	}
	stBad, err := CalendarMCPStatus(info)
	if err != nil {
		t.Fatalf("CalendarMCPStatus with malformed manifest: %v", err)
	}
	if stBad.Installed {
		t.Fatal("expected Installed false when manifest is malformed JSON")
	}
	if err := deregisterServerInManifest(defaultPath, GoogleCalendarMCPServerName); err != nil {
		t.Fatalf("expected nil from deregisterServerInManifest on malformed JSON, got %v", err)
	}
	if err := UninstallCalendarMCP(info); err != nil {
		t.Fatalf("UninstallCalendarMCP with malformed JSON: %v", err)
	}
	after, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(badJSON) {
		t.Fatalf("expected malformed manifest untouched, got %q", string(after))
	}

	// Empty mcpServers object is a no-op for deregisterServerInManifest.
	emptyPath := filepath.Join(home, "empty.json")
	_ = os.WriteFile(emptyPath, []byte(`{}`), 0644)
	if err := deregisterServerInManifest(emptyPath, GoogleCalendarMCPServerName); err != nil {
		t.Fatalf("unexpected error on empty manifest: %v", err)
	}

	// Block schemaDir creation with a regular file to cover error branch in InstallCalendarMCP.
	blockerHome := t.TempDir()
	mcpParent := filepath.Join(blockerHome, ".gemini", "antigravity", "mcp")
	if err := os.MkdirAll(mcpParent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mcpParent, GoogleCalendarMCPServerName), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallCalendarMCP(&platform.Info{HomeDir: blockerHome}); err == nil {
		t.Fatal("expected error when schemaDir path is occupied by a file")
	}
}
