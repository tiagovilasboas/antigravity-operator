package templates

import (
	"encoding/json"
	"regexp"
	"testing"
)

// pinnedNpmPackage matches name@X.Y.Z, scoped or not. npx -y runs whatever the
// registry serves for an unpinned or unclaimed name, so every templated MCP
// server must name an exact version (CI checks that it exists with npm view).
var pinnedNpmPackage = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*@\d+\.\d+\.\d+$`)

func TestDefaultMCPServers_PinNpxPackages(t *testing.T) {
	raw, err := FS.ReadFile("mcps/default-servers.json")
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
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(cfg.MCPServers) == 0 {
		t.Fatal("no MCP servers in template")
	}
	for name, srv := range cfg.MCPServers {
		if srv.Command != "npx" {
			continue
		}
		var pkg string
		for _, a := range srv.Args {
			if len(a) > 0 && a[0] != '-' {
				pkg = a
				break
			}
		}
		if !pinnedNpmPackage.MatchString(pkg) {
			t.Errorf("%s: npx package %q must be pinned as name@X.Y.Z", name, pkg)
		}
	}
}
