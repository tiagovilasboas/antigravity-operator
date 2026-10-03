package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/internal/profile"
	"github.com/tiagoboas/antigravity-operator/internal/session"
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

	// 5. Node e NPX (para MCPs)
	rep.add(checkNode())

	// 6. Gemini API Keys (BYOK)
	rep.add(checkAPIKeys())

	// 7. Integração com Harness Core
	rep.add(checkHarnessCore(info))

	// 8. Memória Operacional de Sessão (.agents/session/)
	rep.add(checkSessionMemory("."))

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
		details += " — Execute 'agyo session compact' para reduzir context bloat"
	}

	return CheckItem{
		Name:    "Session Memory (.agents/session/)",
		Status:  status,
		Details: details,
	}
}
