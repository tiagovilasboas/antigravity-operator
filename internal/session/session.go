package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/templates"
)

// Result resume o que foi criado durante a inicialização.
type Result struct {
	SessionDir string
	Created    []string
	Skipped    []string
}

// Summary sintetiza o estado e tarefas da sessão atual.
type Summary struct {
	SessionDir      string   `json:"session_dir"`
	Objective       string   `json:"objective"`
	Status          string   `json:"status"`
	TotalTasks      int      `json:"total_tasks"`
	DoneTasks       int      `json:"done_tasks"`
	Pending         []string `json:"pending"`
	TotalSizeBytes  int64    `json:"total_size_bytes"`
	EstimatedTokens int      `json:"estimated_tokens"`
	HealthStatus    string   `json:"health_status"` // "Optimal 🟢", "Moderate 🟡", "Bloated 🔴"
}

// Init inicializa a pasta de memória operacional de sessão (.agents/session/) no diretório alvo.
func Init(targetDir string, force bool) (*Result, error) {
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório de sessão: %w", err)
	}

	result := &Result{
		SessionDir: sessionDir,
	}

	// 1. Arquivos canônicos de memória operacional
	files := []struct {
		destName     string
		templatePath string
	}{
		{"state.md", "session/state.md"},
		{"decisions.md", "session/decisions.md"},
		{"todo.md", "session/todo.md"},
	}

	for _, f := range files {
		destPath := filepath.Join(sessionDir, f.destName)
		if fileExists(destPath) && !force {
			result.Skipped = append(result.Skipped, destPath)
			continue
		}

		content, err := templates.FS.ReadFile(f.templatePath)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler template %s: %w", f.templatePath, err)
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return nil, fmt.Errorf("falha ao escrever %s: %w", destPath, err)
		}
		result.Created = append(result.Created, destPath)
	}

	// 2. Criar .gitignore dentro de .agents para proteger logs e dados sensíveis
	agentsGitignore := filepath.Join(targetDir, ".agents", ".gitignore")
	if !fileExists(agentsGitignore) {
		gitignoreContent := []byte("# Proteção de dados operacionais e temporários do agente\n*.log\ntmp/\ncache/\n*.dump\ncredentials*\n")
		_ = os.WriteFile(agentsGitignore, gitignoreContent, 0644)
		result.Created = append(result.Created, agentsGitignore)
	}

	// 3. Criar .agentignore na raiz do projeto para blacklist de contexto
	agentignorePath := filepath.Join(targetDir, ".agentignore")
	if !fileExists(agentignorePath) {
		content, err := templates.FS.ReadFile("session/agentignore")
		if err == nil {
			_ = os.WriteFile(agentignorePath, content, 0644)
			result.Created = append(result.Created, agentignorePath)
		}
	} else if !force {
		result.Skipped = append(result.Skipped, agentignorePath)
	}

	return result, nil
}

// GetSummary extrai um resumo executivo da sessão atual.
func GetSummary(targetDir string) (*Summary, error) {
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	stateFile := filepath.Join(sessionDir, "state.md")
	todoFile := filepath.Join(sessionDir, "todo.md")

	if !fileExists(stateFile) {
		return nil, fmt.Errorf("nenhuma sessão ativa encontrada em %s (execute 'agyo init' primeiro)", sessionDir)
	}

	summary := &Summary{
		SessionDir: sessionDir,
		Objective:  "Não definido",
		Status:     "Desconhecido",
	}

	// 1. Parse do state.md
	if f, err := os.Open(stateFile); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		inObjective := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "## Objetivo Atual") {
				inObjective = true
				continue
			}
			if strings.HasPrefix(line, "## ") && inObjective {
				inObjective = false
			}
			if inObjective && strings.HasPrefix(line, "- ") && summary.Objective == "Não definido" {
				summary.Objective = strings.TrimPrefix(line, "- ")
			}
			if strings.HasPrefix(line, "- **Fase Atual:**") {
				summary.Status = strings.TrimSpace(strings.TrimPrefix(line, "- **Fase Atual:**"))
			}
		}
	}

	// 2. Parse do todo.md
	if f, err := os.Open(todoFile); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
				summary.TotalTasks++
				summary.DoneTasks++
			} else if strings.HasPrefix(line, "- [ ]") {
				summary.TotalTasks++
				taskName := strings.TrimSpace(strings.TrimPrefix(line, "- [ ]"))
				summary.Pending = append(summary.Pending, taskName)
			}
		}
	}

	// 3. Cálculo de pegada de memória e tokens estimados
	var totalBytes int64
	for _, fname := range []string{"state.md", "todo.md", "decisions.md"} {
		if fi, err := os.Stat(filepath.Join(sessionDir, fname)); err == nil {
			totalBytes += fi.Size()
		}
	}
	summary.TotalSizeBytes = totalBytes
	summary.EstimatedTokens = int(totalBytes / 4)

	if summary.EstimatedTokens < 1500 && summary.DoneTasks < 8 {
		summary.HealthStatus = "Optimal 🟢"
	} else if summary.EstimatedTokens < 3500 && summary.DoneTasks < 15 {
		summary.HealthStatus = "Moderate 🟡"
	} else {
		summary.HealthStatus = "Bloated 🔴"
	}

	return summary, nil
}

// Archive compacta e arquiva os arquivos da sessão atual para histórico.
func Archive(targetDir string) (string, error) {
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	stateFile := filepath.Join(sessionDir, "state.md")
	todoFile := filepath.Join(sessionDir, "todo.md")

	if !fileExists(stateFile) {
		return "", fmt.Errorf("nenhuma sessão encontrada para arquivar em %s", sessionDir)
	}

	archiveDir := filepath.Join(sessionDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar pasta de arquivo: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02-150405")
	archiveFile := filepath.Join(archiveDir, fmt.Sprintf("session-%s.md", timestamp))

	stateContent, _ := os.ReadFile(stateFile)
	todoContent, _ := os.ReadFile(todoFile)

	merged := fmt.Sprintf("# Sessão Arquivada em %s\n\n%s\n\n---\n\n%s\n",
		time.Now().Format(time.RFC1123),
		string(stateContent),
		string(todoContent),
	)

	if err := os.WriteFile(archiveFile, []byte(merged), 0644); err != nil {
		return "", fmt.Errorf("falha ao gravar arquivo de histórico: %w", err)
	}

	// Reseta state.md e todo.md com templates limpos
	cleanState, _ := templates.FS.ReadFile("session/state.md")
	cleanTodo, _ := templates.FS.ReadFile("session/todo.md")

	_ = os.WriteFile(stateFile, cleanState, 0644)
	_ = os.WriteFile(todoFile, cleanTodo, 0644)

	return archiveFile, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
