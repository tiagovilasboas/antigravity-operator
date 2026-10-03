package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/templates"
)

const mcpTemplate = "mcps/default-servers.json"

// MCPStatus describes the installed MCP manifest compared to the built-in one.
type MCPStatus struct {
	Path      string
	Exists    bool
	Invalid   bool     // file is not valid JSON
	Unpinned  []string // npx packages without an exact version, "server: pkg"
	Differs   bool     // content differs from the built-in template
	ReadError error
}

// NeedsUpdate reports whether `agyo sync --update-mcp` would change the file.
func (s MCPStatus) NeedsUpdate() bool {
	return s.Exists && (s.Invalid || s.Differs || len(s.Unpinned) > 0)
}

// MCPPath is where sync installs the default MCP manifest.
func MCPPath(info *platform.Info) string {
	return filepath.Join(info.GeminiDir, "mcp", "default-servers.json")
}

type mcpManifest struct {
	MCPServers map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"mcpServers"`
}

// CheckMCP compares the installed manifest with the built-in template. Sync
// never overwrites an existing manifest, so users who installed an older
// release keep its (possibly unpinned or broken) packages until they opt in.
func CheckMCP(info *platform.Info) MCPStatus {
	st := MCPStatus{Path: MCPPath(info)}
	installed, err := os.ReadFile(st.Path)
	if os.IsNotExist(err) {
		return st
	}
	st.Exists = true
	if err != nil {
		st.ReadError = err
		return st
	}
	builtin, err := templates.FS.ReadFile(mcpTemplate)
	if err != nil {
		st.ReadError = err
		return st
	}

	var got, want any
	if json.Unmarshal(installed, &got) != nil {
		st.Invalid = true
		st.Differs = true
		return st
	}
	_ = json.Unmarshal(builtin, &want)
	st.Differs = !reflect.DeepEqual(got, want)

	var m mcpManifest
	_ = json.Unmarshal(installed, &m)
	for name, srv := range m.MCPServers {
		if srv.Command != "npx" {
			continue
		}
		if pkg := templates.NpxPackage(srv.Args); !templates.IsPinnedNpmPackage(pkg) {
			st.Unpinned = append(st.Unpinned, fmt.Sprintf("%s: %s", name, pkg))
		}
	}
	sort.Strings(st.Unpinned)
	return st
}

// UpdateMCP rewrites the installed manifest with the built-in template. An
// existing file that differs is first copied to default-servers.json.bak-<UTC
// timestamp> next to it; the backup path is returned ("" when none was made).
// A file that already matches the template is left untouched.
func UpdateMCP(info *platform.Info, now time.Time) (backup string, changed bool, err error) {
	st := CheckMCP(info)
	if st.ReadError != nil {
		return "", false, fmt.Errorf("falha ao ler manifesto MCP: %w", st.ReadError)
	}
	if st.Exists && !st.Differs {
		return "", false, nil
	}
	builtin, err := templates.FS.ReadFile(mcpTemplate)
	if err != nil {
		return "", false, fmt.Errorf("falha ao ler manifesto MCP embutido: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(st.Path), 0o755); err != nil {
		return "", false, fmt.Errorf("falha ao criar pasta de mcp: %w", err)
	}
	if st.Exists {
		old, err := os.ReadFile(st.Path)
		if err != nil {
			return "", false, fmt.Errorf("falha ao ler manifesto MCP: %w", err)
		}
		backup = st.Path + ".bak-" + now.UTC().Format("20060102T150405Z")
		if err := os.WriteFile(backup, old, 0o600); err != nil {
			return "", false, fmt.Errorf("falha ao gravar backup do manifesto MCP: %w", err)
		}
	}
	if err := os.WriteFile(st.Path, builtin, 0o644); err != nil {
		return backup, false, fmt.Errorf("falha ao gravar manifesto MCP: %w", err)
	}
	return backup, true, nil
}
