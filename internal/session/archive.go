package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/templates"
)

// ArchiveInfo sintetiza uma sessão arquivada encontrada no disco.
type ArchiveInfo struct {
	Filename   string `json:"filename"`
	Path       string `json:"path"`
	Timestamp  string `json:"timestamp"`
	Objective  string `json:"objective"`
	SizeBytes  int64  `json:"size_bytes"`
	TasksTotal int    `json:"tasks_total"`
	TasksDone  int    `json:"tasks_done"`
}

// RestoreResult detalha o resultado de uma operação de restauração de sessão.
type RestoreResult struct {
	RestoredFile string
	BackupFile   string
	Objective    string
	TasksCount   int
}

// ListArchives retorna a lista de todas as sessões arquivadas (.agents/session/archive/session-*.md).
func ListArchives(targetDir string) ([]ArchiveInfo, error) {
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	archiveDir := filepath.Join(sessionDir, "archive")

	if !fileExists(archiveDir) {
		return nil, nil
	}

	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler diretório de arquivos %s: %w", archiveDir, err)
	}

	var archives []ArchiveInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "session-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(archiveDir, entry.Name())
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		info := ArchiveInfo{
			Filename:  entry.Name(),
			Path:      fullPath,
			SizeBytes: fi.Size(),
			Objective: "Não definido",
		}

		// Extrai timestamp do nome do arquivo session-YYYY-MM-DD-HHMMSS.md
		cleanName := strings.TrimPrefix(entry.Name(), "session-")
		cleanName = strings.TrimSuffix(cleanName, ".md")
		info.Timestamp = cleanName

		// Parse do conteúdo para extrair objetivo e tarefas
		if f, err := os.Open(fullPath); err == nil {
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
				if inObjective && strings.HasPrefix(line, "- ") && info.Objective == "Não definido" {
					info.Objective = strings.TrimPrefix(line, "- ")
				}
				if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
					info.TasksTotal++
					info.TasksDone++
				} else if strings.HasPrefix(line, "- [ ]") {
					info.TasksTotal++
				}
			}
			f.Close()
		}

		archives = append(archives, info)
	}

	// Ordena da mais recente para a mais antiga
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Filename > archives[j].Filename
	})

	return archives, nil
}

// Restore restaura uma sessão arquivada para o estado ativo em .agents/session/.
func Restore(targetDir string, archiveName string) (*RestoreResult, error) {
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	archiveDir := filepath.Join(sessionDir, "archive")

	archives, err := ListArchives(targetDir)
	if err != nil {
		return nil, err
	}
	if len(archives) == 0 {
		return nil, fmt.Errorf("nenhuma sessão arquivada encontrada em %s", archiveDir)
	}

	var targetArchive *ArchiveInfo
	if archiveName == "" || archiveName == "latest" {
		targetArchive = &archives[0]
	} else {
		for i := range archives {
			if archives[i].Filename == archiveName || strings.Contains(archives[i].Filename, archiveName) {
				targetArchive = &archives[i]
				break
			}
		}
	}

	if targetArchive == nil {
		return nil, fmt.Errorf("arquivo de sessão '%s' não encontrado em %s", archiveName, archiveDir)
	}

	archiveContent, err := os.ReadFile(targetArchive.Path)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler arquivo de sessão %s: %w", targetArchive.Path, err)
	}

	raw := string(archiveContent)
	// Formato gerado pelo Archive():
	// # Sessão Arquivada em <date>\n\n<stateContent>\n\n---\n\n<todoContent>\n
	parts := strings.Split(raw, "\n\n---\n\n")
	if len(parts) < 2 {
		return nil, fmt.Errorf("formato de arquivo de sessão inválido em %s (divisor '---' não encontrado)", targetArchive.Filename)
	}

	stateRaw := parts[0]
	todoRaw := parts[1]

	// Remove cabeçalho inicial de arquivamento "# Sessão Arquivada em ...\n\n" se presente
	if idx := strings.Index(stateRaw, "# Estado da Sessão"); idx != -1 {
		stateRaw = stateRaw[idx:]
	} else if idx := strings.Index(stateRaw, "\n\n"); idx != -1 {
		stateRaw = stateRaw[idx+2:]
	}

	// 1. Cria backup de segurança do estado ativo atual antes de sobrescrever
	stateFile := filepath.Join(sessionDir, "state.md")
	todoFile := filepath.Join(sessionDir, "todo.md")

	currentState, _ := os.ReadFile(stateFile)
	currentTodo, _ := os.ReadFile(todoFile)

	backupFileName := fmt.Sprintf("pre-restore-%s.md", time.Now().Format("2006-01-02-150405"))
	backupPath := filepath.Join(archiveDir, backupFileName)

	preRestoreMerged := fmt.Sprintf("# Backup Pré-Restauração em %s\n\n%s\n\n---\n\n%s\n",
		time.Now().Format(time.RFC1123), string(currentState), string(currentTodo))
	_ = os.WriteFile(backupPath, []byte(preRestoreMerged), 0644)

	// 2. Sobrescreve state.md e todo.md com os conteúdos restaurados
	if err := os.WriteFile(stateFile, []byte(stateRaw), 0644); err != nil {
		return nil, fmt.Errorf("falha ao restaurar %s: %w", stateFile, err)
	}
	if err := os.WriteFile(todoFile, []byte(todoRaw), 0644); err != nil {
		return nil, fmt.Errorf("falha ao restaurar %s: %w", todoFile, err)
	}

	// 3. Garante que decisions.md existe
	decisionsFile := filepath.Join(sessionDir, "decisions.md")
	if !fileExists(decisionsFile) {
		cleanDecisions, _ := templates.FS.ReadFile("session/decisions.md")
		_ = os.WriteFile(decisionsFile, cleanDecisions, 0644)
	}

	return &RestoreResult{
		RestoredFile: targetArchive.Filename,
		BackupFile:   backupFileName,
		Objective:    targetArchive.Objective,
		TasksCount:   targetArchive.TasksTotal,
	}, nil
}
