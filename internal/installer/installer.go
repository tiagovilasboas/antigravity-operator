package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/templates"
)

// SyncResult reporta as ações executadas pelo instalador.
type SyncResult struct {
	RulesPath  string
	MCPPath    string
	SkillsPath string
	Updated    []string
	Skipped    []string
	Backups    []string
}

// writeSafeFile grava o arquivo com segurança:
// 1. Não escreve se o arquivo ou seu diretório pai for symlink (preserva repositórios externos como harness-core).
// 2. Se o arquivo já existir com conteúdo idêntico, não faz nada.
// 3. Se o arquivo já existir e diferir, cria backup datado (.bak-<timestamp>) antes de sobrescrever.
func writeSafeFile(targetPath string, content []byte, perm os.FileMode, now time.Time) (wrote bool, backup string, skippedSymlink bool, err error) {
	parentDir := filepath.Dir(targetPath)
	if pfi, lerr := os.Lstat(parentDir); lerr == nil && (pfi.Mode()&os.ModeSymlink != 0) {
		return false, "", true, nil
	}

	fi, lerr := os.Lstat(targetPath)
	if lerr == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return false, "", true, nil
		}
		existing, rerr := os.ReadFile(targetPath)
		if rerr == nil && bytes.Equal(existing, content) {
			return false, "", false, nil
		}
		backup = targetPath + ".bak-" + now.UTC().Format("20060102T150405Z")
		if berr := os.WriteFile(backup, existing, 0600); berr != nil {
			return false, "", false, fmt.Errorf("falha ao criar backup de %s: %w", targetPath, berr)
		}
	} else if !os.IsNotExist(lerr) {
		return false, "", false, fmt.Errorf("falha ao inspecionar %s: %w", targetPath, lerr)
	}

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return false, "", false, fmt.Errorf("falha ao criar pasta %s: %w", parentDir, err)
	}

	if err := os.WriteFile(targetPath, content, perm); err != nil {
		return false, backup, false, fmt.Errorf("falha ao gravar %s: %w", targetPath, err)
	}

	return true, backup, false, nil
}

// Sync instala ou atualiza as configurações do Antigravity na máquina local.
func Sync(info *platform.Info) (*SyncResult, error) {
	result := &SyncResult{}
	now := time.Now()

	// 1. Assegurar diretórios base
	rulesDir := filepath.Join(info.GeminiDir, "rules")
	mcpDir := filepath.Join(info.GeminiDir, "mcp")

	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta de rules: %w", err)
	}
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta de mcp: %w", err)
	}

	// 2. Instalar regra canônica do Session Agent
	ruleContent, err := templates.FS.ReadFile("rules/session-agent.md")
	if err != nil {
		return nil, fmt.Errorf("falha ao ler regra embutida: %w", err)
	}

	sessionRuleTarget := filepath.Join(rulesDir, "session-agent.md")
	result.RulesPath = sessionRuleTarget
	wrote, backup, skipped, err := writeSafeFile(sessionRuleTarget, ruleContent, 0644, now)
	if err != nil {
		return nil, fmt.Errorf("falha ao gravar regra do session agent: %w", err)
	}
	if backup != "" {
		result.Backups = append(result.Backups, backup)
	}
	if skipped {
		result.Skipped = append(result.Skipped, sessionRuleTarget)
	} else if wrote {
		result.Updated = append(result.Updated, sessionRuleTarget)
	}

	// 3. MCPs padrão
	mcpDefaultTarget := filepath.Join(mcpDir, "default-servers.json")
	if _, err := os.Stat(mcpDefaultTarget); os.IsNotExist(err) {
		mcpContent, err := templates.FS.ReadFile("mcps/default-servers.json")
		if err == nil {
			_ = os.WriteFile(mcpDefaultTarget, mcpContent, 0644)
			result.MCPPath = mcpDefaultTarget
			result.Updated = append(result.Updated, mcpDefaultTarget)
		}
	} else {
		result.MCPPath = mcpDefaultTarget
	}

	// 3.1 Registra notebooklm no mcp_config.json ativo do Antigravity se existir
	homeDir := info.HomeDir
	if homeDir == "" {
		homeDir = filepath.Dir(filepath.Dir(info.GeminiDir))
	}
	activeConfigPaths := []string{
		filepath.Join(info.GeminiDir, "mcp_config.json"),
		filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
	}
	for _, cfgPath := range activeConfigPaths {
		if fi, err := os.Stat(cfgPath); err == nil && !fi.IsDir() {
			if err := RegisterNotebookLMMCP(cfgPath); err == nil {
				result.Updated = append(result.Updated, cfgPath)
			}
		}
	}

	// 4. Instalar todas as skills embutidas (agyo + student pack)
	homeDir = info.HomeDir
	if homeDir == "" {
		homeDir = filepath.Dir(filepath.Dir(info.GeminiDir))
	}
	skillsBaseDir := filepath.Join(homeDir, ".gemini", "config", "skills")

	_ = fs.WalkDir(templates.FS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		relPath, rerr := filepath.Rel("skills", path)
		if rerr != nil {
			return nil
		}
		targetPath := filepath.Join(skillsBaseDir, relPath)
		if relPath == filepath.Join("agyo", "SKILL.md") {
			result.SkillsPath = targetPath
		}
		content, rerr := templates.FS.ReadFile(path)
		if rerr != nil {
			return nil
		}
		wrote, backup, skipped, werr := writeSafeFile(targetPath, content, 0644, now)
		if werr == nil {
			if backup != "" {
				result.Backups = append(result.Backups, backup)
			}
			if skipped {
				result.Skipped = append(result.Skipped, targetPath)
			} else if wrote {
				result.Updated = append(result.Updated, targetPath)
			}
		}
		return nil
	})

	return result, nil
}

// RegisterNotebookLMMCP assegura que a entrada do servidor MCP notebooklm esteja configurada no arquivo informado.
func RegisterNotebookLMMCP(mcpConfigPath string) error {
	data, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		return err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	servers, ok := root["mcpServers"].(map[string]interface{})
	if !ok || servers == nil {
		servers = make(map[string]interface{})
		root["mcpServers"] = servers
	}

	if _, exists := servers["notebooklm"]; !exists {
		servers["notebooklm"] = map[string]interface{}{
			"command": "agyo",
			"args":    []string{"notebooklm", "mcp"},
		}

		updatedBytes, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return err
		}

		return os.WriteFile(mcpConfigPath, updatedBytes, 0644)
	}

	return nil
}
