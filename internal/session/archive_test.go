package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

func TestListArchivesAndRestore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-archive-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = session.Init(tempDir, false)
	if err != nil {
		t.Fatalf("Init falhou: %v", err)
	}

	sessionDir := filepath.Join(tempDir, ".agents", "session")

	// 1. Sem arquivos arquivados inicialmente
	archivesEmpty, err := session.ListArchives(tempDir)
	if err != nil {
		t.Fatalf("ListArchives falhou em diretório vazio: %v", err)
	}
	if len(archivesEmpty) != 0 {
		t.Errorf("esperava 0 arquivos, obteve %d", len(archivesEmpty))
	}

	// 2. Criar uma sessão customizada e arquivar
	statePath := filepath.Join(sessionDir, "state.md")
	todoPath := filepath.Join(sessionDir, "todo.md")

	customState := `# Estado da Sessão
## Objetivo Atual
- Construir Feature Alpha
## Status em Tempo Real
- Fase Atual: Concluída
`
	customTodo := `# Lista de Tarefas da Sessão
## Pendentes 📋
- [ ] Tarefa futura
## Concluídas ✅
- [x] Tarefa Alpha 1
- [x] Tarefa Alpha 2
`
	_ = os.WriteFile(statePath, []byte(customState), 0644)
	_ = os.WriteFile(todoPath, []byte(customTodo), 0644)

	archivePath, err := session.Archive(tempDir)
	if err != nil {
		t.Fatalf("Archive falhou: %v", err)
	}

	// 3. Validar ListArchives
	archives, err := session.ListArchives(tempDir)
	if err != nil {
		t.Fatalf("ListArchives falhou após arquivamento: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("esperava 1 arquivo arquivado, obteve %d", len(archives))
	}
	if archives[0].Objective != "Construir Feature Alpha" {
		t.Errorf("esperava objetivo 'Construir Feature Alpha', obteve %q", archives[0].Objective)
	}
	if archives[0].TasksDone != 2 {
		t.Errorf("esperava 2 tarefas concluídas, obteve %d", archives[0].TasksDone)
	}

	// 4. Testar Restore (restaurar a sessão arquivada)
	// Primeiro, altera o estado ativo para simular nova sessão
	_ = os.WriteFile(statePath, []byte("# Estado Ativo Novo\n## Objetivo Atual\n- Feature Beta\n"), 0644)

	resRestore, err := session.Restore(tempDir, "latest")
	if err != nil {
		t.Fatalf("Restore falhou: %v", err)
	}

	if resRestore.Objective != "Construir Feature Alpha" {
		t.Errorf("esperava objetivo restaurado 'Construir Feature Alpha', obteve %q", resRestore.Objective)
	}

	// Verifica se state.md ativo foi restaurado com o conteúdo antigo
	restoredStateBytes, _ := os.ReadFile(statePath)
	if !strings.Contains(string(restoredStateBytes), "Construir Feature Alpha") {
		t.Errorf("state.md não foi restaurado corretamente: %s", string(restoredStateBytes))
	}

	// Verifica se backup de segurança pré-restore foi criado
	backupPath := filepath.Join(sessionDir, "archive", resRestore.BackupFile)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("backup de segurança pré-restore não foi criado: %s", backupPath)
	}

	_ = archivePath
}
