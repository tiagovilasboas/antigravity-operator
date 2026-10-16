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
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

// CheckItem representa o resultado de uma verificação individual.
type CheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "OK", "WARN", "FAIL", "INFO"
	Details string `json:"details"`
	// PublicDetails, when set, replaces Details on surfaces served over HTTP
	// (dashboard) because Details carries personal data such as the git email.
	PublicDetails string `json:"-"`
}

// Report agrega todas as verificações do sistema.
type Report struct {
	Platform *platform.Info `json:"platform"`
	Checks   []CheckItem    `json:"checks"`
}

// FixResult sintetiza as ações de autorrecuperação executadas.
type FixResult struct {
	Repaired []string `json:"repaired"`
	Skipped  []string `json:"skipped"`
}

// Fix repara preventivamente elementos corrompidos ou ausentes do ambiente de sessão.
func Fix(targetDir string, info *platform.Info) (*FixResult, error) {
	res := &FixResult{}

	// 1. Repara e garante .agents/session/ e templates ausentes
	initRes, err := session.Init(targetDir, false)
	if err != nil {
		return nil, fmt.Errorf("falha ao auto-reparar memória de sessão: %w", err)
	}
	res.Repaired = append(res.Repaired, initRes.Created...)
	res.Skipped = append(res.Skipped, initRes.Skipped...)

	// 2. Sincroniza regras, MCPs e skills embutidas
	syncRes, err := installer.Sync(info)
	if err == nil {
		res.Repaired = append(res.Repaired, syncRes.Updated...)
		res.Skipped = append(res.Skipped, syncRes.Skipped...)
	}

	return res, nil
}

// Run executa todas as verificações de diagnóstico.
func Run(info *platform.Info) *Report {
	rep := &Report{Platform: info}

	// 1. Git e Configuração de Usuário
	rep.add(checkGit())

	// 2. Google Antigravity IDE & Engine
	rep.add(checkAntigravity(info))

	// 3. Google Chrome / Chromium
	rep.add(checkChrome(info))

	// 4. Porta DevTools & Perfil Isolado
	rep.add(checkDevTools(info))

	// 4b. Multi-Profile Intelligence (Perfis Isolados e do Sistema)
	rep.add(checkChromeProfiles(info))

	// 5. Node e NPX (para MCPs)
	rep.add(checkNode())

	// 5b. Manifesto MCP instalado vs. template embutido
	rep.add(checkMCPManifest(info))

	// 6. Gemini API Keys (BYOK)
	rep.add(checkAPIKeys())

	// 7. Integração com Harness Core
	rep.add(checkHarnessCore(info))

	// 7b. Skills de Agentes
	rep.add(checkSkills(info))

	// 8. Google NotebookLM Readiness
	rep.add(checkNotebookLM(info))

	// 8b. Google Workspace Bridge (Calendar e Docs)
	rep.add(checkGoogleWorkspace(info))

	// 9. Memória Operacional de Sessão (.agents/session/)
	rep.add(checkSessionMemory("."))

	// 10. Lifecycle Hooks de Sessão (Google Antigravity hooks.json)
	rep.add(checkLifecycleHooks(info))

	return rep
}

func (r *Report) add(items ...CheckItem) {
	r.Checks = append(r.Checks, items...)
}

func checkGit() CheckItem {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return CheckItem{Name: "Git", Status: "FAIL", Details: "git não encontrado no PATH"}
	}
	version := strings.TrimSpace(string(out))

	nameOut, _ := exec.Command("git", "config", "user.name").Output()
	emailOut, _ := exec.Command("git", "config", "user.email").Output()

	name := strings.TrimSpace(string(nameOut))
	email := strings.TrimSpace(string(emailOut))

	if name == "" || email == "" {
		return CheckItem{
			Name:    "Git Identity",
			Status:  "WARN",
			Details: fmt.Sprintf("%s (user.name ou user.email não configurados)", version),
		}
	}

	return CheckItem{
		Name:    "Git",
		Status:  "OK",
		Details: fmt.Sprintf("%s (%s <%s>)", version, name, email),
		// Name and email stay in the local CLI output only.
		PublicDetails: fmt.Sprintf("%s (identity configured)", version),
	}
}

func checkChrome(info *platform.Info) CheckItem {
	if info.ChromeBin == "" {
		return CheckItem{
			Name:    "Google Chrome",
			Status:  "FAIL",
			Details: "Nenhum binário de Chrome/Chromium encontrado no sistema",
		}
	}
	return CheckItem{
		Name:    "Google Chrome",
		Status:  "OK",
		Details: fmt.Sprintf("Localizado em: %s", info.ChromeBin),
	}
}

func checkDevTools(info *platform.Info) CheckItem {
	st := profile.CheckStatus(info.BrowserProfile)
	if st.IsRunning {
		return CheckItem{
			Name:    "Chrome DevTools (Port 9222)",
			Status:  "OK",
			Details: fmt.Sprintf("Ativo (%s) no perfil isolado", st.Version),
		}
	}
	return CheckItem{
		Name:    "Chrome DevTools (Port 9222)",
		Status:  "WARN",
		Details: "Inativo (execute 'agyo browser start' para iniciar)",
	}
}

func checkChromeProfiles(info *platform.Info) CheckItem {
	var all []profile.ChromeProfile
	if iso, err := profile.DiscoverProfiles(info.BrowserProfile, "isolated"); err == nil {
		all = append(all, iso...)
	}
	if sys, err := profile.DiscoverProfiles(info.SystemChromeDir, "system"); err == nil {
		all = append(all, sys...)
	}

	if len(all) == 0 {
		return CheckItem{
			Name:    "Chrome Profiles",
			Status:  "INFO",
			Details: "Nenhum diretório de perfil Local State encontrado",
		}
	}

	personalCount := 0
	workspaceCount := 0
	ghostCount := 0
	var domains []string
	seenDomains := make(map[string]bool)

	for _, p := range all {
		switch p.Kind {
		case profile.ProfileKindPersonal:
			personalCount++
		case profile.ProfileKindWorkspace:
			workspaceCount++
			if p.HostedDomain != "" && !seenDomains[p.HostedDomain] {
				seenDomains[p.HostedDomain] = true
				domains = append(domains, p.HostedDomain)
			}
		case profile.ProfileKindGhost:
			ghostCount++
		}
	}

	publicSummary := fmt.Sprintf("%d personal, %d workspace, %d ghost (use 'agyo browser profiles')", personalCount, workspaceCount, ghostCount)
	detailSummary := publicSummary
	if len(domains) > 0 {
		detailSummary = fmt.Sprintf("%d personal, %d workspace [%s], %d ghost (use 'agyo browser profiles')", personalCount, workspaceCount, strings.Join(domains, ", "), ghostCount)
	}

	return CheckItem{
		Name:          "Chrome Profiles",
		Status:        "OK",
		Details:       detailSummary,
		PublicDetails: publicSummary,
	}
}

func checkNode() CheckItem {
	out, err := exec.Command("npx", "--version").Output()
	if err != nil {
		return CheckItem{
			Name:    "NPX (MCP Runtime)",
			Status:  "WARN",
			Details: "npx não encontrado. MCP servers baseados em Node podem falhar",
		}
	}
	return CheckItem{
		Name:    "NPX (MCP Runtime)",
		Status:  "OK",
		Details: fmt.Sprintf("Versão %s disponível", strings.TrimSpace(string(out))),
	}
}

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

func checkAntigravity(info *platform.Info) CheckItem {
	out, err := exec.Command("pgrep", "-i", "antigravity").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		pids := strings.Fields(strings.TrimSpace(string(out)))
		return CheckItem{
			Name:    "Google Antigravity",
			Status:  "OK",
			Details: fmt.Sprintf("Ativo (%d processos detectados, PID primário: %s)", len(pids), pids[0]),
		}
	}

	if info.OS == "darwin" {
		if _, err := os.Stat("/Applications/Antigravity.app"); err == nil {
			return CheckItem{
				Name:    "Google Antigravity",
				Status:  "INFO",
				Details: "Instalado em /Applications/Antigravity.app (inativo)",
			}
		}
	}

	return CheckItem{
		Name:    "Google Antigravity",
		Status:  "INFO",
		Details: "Não detectado em execução no momento",
	}
}

func checkAPIKeys() CheckItem {
	key := os.Getenv("GEMINI_API_KEY")
	source := "GEMINI_API_KEY"
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
		source = "GOOGLE_API_KEY"
	}

	if key != "" {
		masked := key
		if len(key) > 8 {
			masked = key[:4] + "..." + key[len(key)-4:]
		}
		return CheckItem{
			Name:    "Gemini API Key (BYOK)",
			Status:  "OK",
			Details: fmt.Sprintf("Configurada via %s (%s)", source, masked),
			// No key characters over HTTP; the masked value stays in the local CLI.
			PublicDetails: fmt.Sprintf("Configured via %s", source),
		}
	}

	return CheckItem{
		Name:    "Gemini API Key (BYOK)",
		Status:  "INFO",
		Details: "Não definida no ambiente (opcional para MCPs externos)",
	}
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
