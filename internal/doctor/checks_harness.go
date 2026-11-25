package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/hook"
	"github.com/tiagovilasboas/antigravity-operator/internal/installer"
	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

func checkHarnessCore(info *platform.Info) CheckItem {
	if info.HarnessCore == "" {
		return CheckItem{
			Name:    "Harness Core",
			Status:  "INFO",
			Details: "Modo Standalone (regras e templates embutidos)",
		}
	}
	if out, err := exec.Command("git", "-C", info.HarnessCore, "rev-parse", "--is-inside-work-tree").Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		return CheckItem{
			Name:    "Harness Core",
			Status:  "OK",
			Details: fmt.Sprintf("Conectado em %s", info.HarnessCore),
		}
	}
	return CheckItem{
		Name:    "Harness Core",
		Status:  "INFO",
		Details: "Modo Standalone (fonte central não detectada)",
	}
}

func checkMCPManifest(info *platform.Info) CheckItem {
	const name = "MCP Manifest"
	const fix = " Run 'agyo sync --update-mcp' (backs up the current file)."
	st := installer.CheckMCP(info)
	switch {
	case !st.Exists:
		return CheckItem{Name: name, Status: "INFO", Details: "Not installed. Run 'agyo sync'."}
	case st.ReadError != nil:
		return CheckItem{Name: name, Status: "WARN", Details: "Unreadable: " + st.ReadError.Error(), PublicDetails: "Unreadable"}
	case st.Invalid:
		return CheckItem{Name: name, Status: "WARN", Details: "Not valid JSON." + fix}
	case len(st.Unpinned) > 0:
		return CheckItem{Name: name, Status: "WARN", Details: fmt.Sprintf("%d unpinned npx package(s) (%s).%s", len(st.Unpinned), strings.Join(st.Unpinned, "; "), fix)}
	case st.Differs:
		return CheckItem{Name: name, Status: "WARN", Details: "Differs from the built-in template." + fix}
	}
	return CheckItem{Name: name, Status: "OK", Details: "Matches the built-in template (packages pinned)"}
}

func checkSessionMemory(dir string) CheckItem {
	sum, err := session.GetSummary(dir)
	if err != nil {
		return CheckItem{
			Name:    "Session Memory (.agents/session/)",
			Status:  "INFO",
			Details: "Nenhuma sessão ativa neste diretório (execute 'agyo init' para inicializar)",
		}
	}

	details := fmt.Sprintf("Objetivo: %s | %d/%d tarefas | %d B (~%d tokens) [%s]",
		sum.Objective, sum.DoneTasks, sum.TotalTasks, sum.TotalSizeBytes, sum.EstimatedTokens, sum.HealthStatus)

	status := "OK"
	if sum.HealthStatus == "Bloated 🔴" {
		status = "WARN"
		details += " - Execute 'agyo session compact' para reduzir context bloat"
	}

	return CheckItem{
		Name:    "Session Memory (.agents/session/)",
		Status:  status,
		Details: details,
	}
}

func checkGoogleWorkspace(info *platform.Info) CheckItem {
	const name = "Google Workspace Bridge"
	geminiDir := ""
	if info != nil {
		geminiDir = info.GeminiDir
	}
	if geminiDir == "" {
		home, _ := os.UserHomeDir()
		geminiDir = filepath.Join(home, ".gemini", "antigravity")
	}
	authPath := workspace.AuthFilePathForContext(geminiDir, os.Getenv("AGYO_GOOGLE_CONTEXT"))
	tok, err := workspace.LoadToken(authPath)
	if err != nil {
		return CheckItem{
			Name:    name,
			Status:  "INFO",
			Details: "Não autenticado (execute: agyo google auth)",
		}
	}
	if tok.IsExpired() {
		if tok.RefreshToken != "" {
			return CheckItem{
				Name:    name,
				Status:  "OK",
				Details: "Autenticado (token expirado com auto-refresh configurado)",
			}
		}
		return CheckItem{
			Name:    name,
			Status:  "WARN",
			Details: "Token expirado sem refresh token (execute: agyo google auth)",
		}
	}
	return CheckItem{
		Name:    name,
		Status:  "OK",
		Details: "Autenticado e pronto para Calendar e Docs",
	}
}

func checkNotebookLM(info *platform.Info) CheckItem {
	st := notebook.CheckSession(info, 0)
	if !st.ChromeRunning {
		return CheckItem{
			Name:    "Google NotebookLM",
			Status:  "INFO",
			Details: "Chrome DevTools inativo (use 'agyo notebook open' para conectar)",
		}
	}
	if !st.HasTab {
		return CheckItem{
			Name:    "Google NotebookLM",
			Status:  "INFO",
			Details: "Chrome ativo, sem aba do NotebookLM (use 'agyo notebook open')",
		}
	}
	if !st.IsLoggedIn {
		return CheckItem{
			Name:    "Google NotebookLM",
			Status:  "WARN",
			Details: "Aba aberta aguardando login na conta Google",
		}
	}
	return CheckItem{
		Name:    "Google NotebookLM",
		Status:  "OK",
		Details: "Conectado e autenticado",
	}
}

func checkSkills(info *platform.Info) CheckItem {
	const name = "Agent Skills"
	home := info.HomeDir
	if home == "" {
		h, err := os.UserHomeDir()
		if err == nil {
			home = h
		}
	}
	if home == "" {
		return CheckItem{Name: name, Status: "INFO", Details: "Diretório home não detectado"}
	}

	skillsDir := filepath.Join(home, ".gemini", "config", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckItem{Name: name, Status: "INFO", Details: "Diretório de skills (~/.gemini/config/skills) não encontrado"}
		}
		return CheckItem{Name: name, Status: "WARN", Details: fmt.Sprintf("Falha ao ler %s: %v", skillsDir, err)}
	}

	var stale []string
	activeCount := 0
	for _, entry := range entries {
		entryName := entry.Name()
		if strings.HasPrefix(entryName, ".") {
			continue
		}
		if strings.HasSuffix(entryName, ".bak") || strings.HasSuffix(entryName, ".backup") || strings.HasSuffix(entryName, ".old") || strings.HasSuffix(entryName, ".tmp") {
			stale = append(stale, entryName)
			continue
		}
		activeCount++
	}

	if len(stale) > 0 {
		return CheckItem{
			Name:   name,
			Status: "WARN",
			Details: fmt.Sprintf("%d skills ativas; %d pasta(s) de backup/stale encontradas (%s). Mova pastas .bak para fora de %s para evitar carregamento duplicado",
				activeCount, len(stale), strings.Join(stale, ", "), skillsDir),
		}
	}

	return CheckItem{
		Name:    name,
		Status:  "OK",
		Details: fmt.Sprintf("%d skills ativas registradas em %s (sem pastas .bak)", activeCount, skillsDir),
	}
}

func checkLifecycleHooks(info *platform.Info) CheckItem {
	name := "Lifecycle Hooks"
	active, targetFile, err := hook.AntigravityHookStatus("user", "")
	if err != nil {
		return CheckItem{
			Name:    name,
			Status:  "WARN",
			Details: fmt.Sprintf("Erro ao inspecionar %s: %v", targetFile, err),
		}
	}
	if !active {
		return CheckItem{
			Name:    name,
			Status:  "INFO",
			Details: fmt.Sprintf("Não instalados em %s (execute: agyo hook antigravity install)", targetFile),
		}
	}
	return CheckItem{
		Name:    name,
		Status:  "OK",
		Details: fmt.Sprintf("agyo-session-agent ativo em %s", targetFile),
	}
}
