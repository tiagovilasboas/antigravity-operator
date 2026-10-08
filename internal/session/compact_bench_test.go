package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

func BenchmarkCompact_100Tasks(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "compact-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	sessionDir := filepath.Join(tempDir, ".agents", "session")
	_ = os.MkdirAll(sessionDir, 0755)

	todoPath := filepath.Join(sessionDir, "todo.md")
	statePath := filepath.Join(sessionDir, "state.md")

	var taskLines []string
	taskLines = append(taskLines, "# Lista de Tarefas")
	taskLines = append(taskLines, "## Pendentes")
	taskLines = append(taskLines, "- [ ] Pendente 1")
	taskLines = append(taskLines, "## Concluídas")
	for i := 1; i <= 100; i++ {
		taskLines = append(taskLines, fmt.Sprintf("- [x] Tarefa finalizada %d com detalhes de teste", i))
	}
	todoTemplate := strings.Join(taskLines, "\n")
	stateTemplate := "# Estado\n## Objetivo\n- Teste\n## Status\n- Ok\n## Histórico\n- Turno 1\n- Turno 2\n"

	opts := session.CompactOptions{
		Threshold: 5,
		KeepLast:  3,
		DryRun:    false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		_ = os.WriteFile(todoPath, []byte(todoTemplate), 0644)
		_ = os.WriteFile(statePath, []byte(stateTemplate), 0644)
		b.StartTimer()

		_, err := session.Compact(tempDir, opts)
		if err != nil {
			b.Fatalf("erro no compact benchmark: %v", err)
		}
	}
}
