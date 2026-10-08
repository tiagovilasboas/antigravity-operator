package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestSync(t *testing.T) {
	tempGemini := filepath.Join(t.TempDir(), ".gemini", "antigravity")
	info := &platform.Info{
		GeminiDir: tempGemini,
	}

	// 1. Initial Sync
	res, err := Sync(info)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if res.RulesPath == "" {
		t.Error("expected non-empty RulesPath")
	}

	if res.MCPPath == "" {
		t.Error("expected non-empty MCPPath")
	}

	if res.SkillsPath == "" {
		t.Error("expected non-empty SkillsPath")
	}

	// Verify files actually exist on disk
	if _, err := os.Stat(res.RulesPath); os.IsNotExist(err) {
		t.Errorf("expected rules file at %s, but does not exist", res.RulesPath)
	}

	ruleBytes, err := os.ReadFile(res.RulesPath)
	if err != nil || len(ruleBytes) == 0 {
		t.Error("rules file is empty or unreadable")
	}

	if _, err := os.Stat(res.MCPPath); os.IsNotExist(err) {
		t.Errorf("expected MCP file at %s, but does not exist", res.MCPPath)
	}

	if _, err := os.Stat(res.SkillsPath); os.IsNotExist(err) {
		t.Errorf("expected skills file at %s, but does not exist", res.SkillsPath)
	}

	// 2. Idempotent Sync (should preserve existing files without errors)
	res2, err := Sync(info)
	if err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	if res2.RulesPath != res.RulesPath {
		t.Errorf("expected same RulesPath, got %s", res2.RulesPath)
	}
	if res2.MCPPath != res.MCPPath {
		t.Errorf("expected same MCPPath, got %s", res2.MCPPath)
	}
}

func TestSync_CreateDirectoryError(t *testing.T) {
	// Point to an invalid path that cannot be created (e.g., child of a regular file)
	tmpFile := filepath.Join(t.TempDir(), "blocker")
	_ = os.WriteFile(tmpFile, []byte("blocker"), 0644)

	invalidInfo := &platform.Info{
		GeminiDir: filepath.Join(tmpFile, "invalid_subdir"),
	}

	_, err := Sync(invalidInfo)
	if err == nil {
		t.Error("expected error when GeminiDir cannot be created, got nil")
	}
}

func TestRegisterNotebookLMMCP(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "mcp_config.json")

	initialJSON := []byte(`{
  "mcpServers": {
    "playwright": {
      "command": "npx"
    }
  }
}`)
	if err := os.WriteFile(cfgPath, initialJSON, 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	if err := RegisterNotebookLMMCP(cfgPath); err != nil {
		t.Fatalf("RegisterNotebookLMMCP failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read updated config: %v", err)
	}

	if !os.FileMode(0644).IsDir() && (!stringContains(string(data), "notebooklm") || !stringContains(string(data), "agyo")) {
		t.Errorf("expected updated config to contain 'notebooklm' and 'agyo', got: %s", string(data))
	}
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && bytesContains([]byte(s), []byte(substr)))
}

func bytesContains(b, sub []byte) bool {
	for i := 0; i+len(sub) <= len(b); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
