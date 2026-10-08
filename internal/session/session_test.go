package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

func TestInit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Primeira execução: deve criar os arquivos
	res, err := session.Init(tempDir, false)
	if err != nil {
		t.Fatalf("esperava sucesso no Init, obteve erro: %v", err)
	}

	if len(res.Created) == 0 {
		t.Errorf("esperava arquivos criados na primeira execução")
	}

	expectedFiles := []string{
		filepath.Join(tempDir, ".agents", "session", "state.md"),
		filepath.Join(tempDir, ".agents", "session", "decisions.md"),
		filepath.Join(tempDir, ".agents", "session", "todo.md"),
		filepath.Join(tempDir, ".agents", ".gitignore"),
		filepath.Join(tempDir, ".agentignore"),
	}

	for _, ef := range expectedFiles {
		if _, err := os.Stat(ef); os.IsNotExist(err) {
			t.Errorf("arquivo esperado não foi criado: %s", ef)
		}
	}

	// 2. Segunda execução sem force: deve pular arquivos existentes (idempotência)
	res2, err := session.Init(tempDir, false)
	if err != nil {
		t.Fatalf("esperava sucesso no segundo Init, obteve erro: %v", err)
	}

	if len(res2.Skipped) < 3 {
		t.Errorf("esperava pelo menos 3 arquivos ignorados por já existirem, obteve: %d", len(res2.Skipped))
	}
}

func TestGetSummaryAndArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-summary-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = session.Init(tempDir, false)
	if err != nil {
		t.Fatalf("Init falhou: %v", err)
	}

	// Testar GetSummary
	summary, err := session.GetSummary(tempDir)
	if err != nil {
		t.Fatalf("GetSummary falhou: %v", err)
	}

	if summary == nil {
		t.Fatalf("esperava summary preenchido")
	}

	if summary.TotalTasks == 0 {
		t.Errorf("esperava pelo menos 1 tarefa no template padrão")
	}

	// Testar Archive
	archivePath, err := session.Archive(tempDir)
	if err != nil {
		t.Fatalf("Archive falhou: %v", err)
	}

	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Errorf("arquivo de arquivo não foi criado: %s", archivePath)
	}
}
