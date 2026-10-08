package templates

import (
	"encoding/json"
	"io/fs"
	"testing"
)

func TestMCPTemplates_PinNpxPackages(t *testing.T) {
	files, err := fs.Glob(FS, "mcps/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no MCP templates found in mcps/")
	}
	for _, f := range files {
		raw, err := FS.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			MCPServers map[string]struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("%s: invalid JSON: %v", f, err)
		}
		if len(cfg.MCPServers) == 0 {
			t.Errorf("%s: no MCP servers", f)
		}
		for name, srv := range cfg.MCPServers {
			if srv.Command != "npx" {
				continue
			}
			pkg := NpxPackage(srv.Args)
			if !IsPinnedNpmPackage(pkg) {
				t.Errorf("%s: %s: npx package %q must be pinned as name@X.Y.Z", f, name, pkg)
			}
		}
	}
}
