package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportMarkdown(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, ".agents", "session")
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		t.Fatalf("falha ao criar pasta de sessão: %v", err)
	}

	stateContent := "# Estado da Sessão\n## Objetivo Atual\n- Testar exportador\n## Status em Tempo Real\n- **Fase Atual:** Execução"
	os.WriteFile(filepath.Join(sessionDir, "state.md"), []byte(stateContent), 0644)

	todoContent := "# Lista de Tarefas\n- [x] Fazer setup\n- [ ] Implementar relatorio"
	os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(todoContent), 0644)

	decisionsContent := "# Decisões\n### Usar Go puro\nDecidimos usar text/template e html/template para evitar libs externas.\nCom isso o código fica mais leve."
	os.WriteFile(filepath.Join(sessionDir, "decisions.md"), []byte(decisionsContent), 0644)

	opts := ExportOptions{
		Format:    "markdown",
		TargetDir: tempDir,
	}

	result, err := Export(opts)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if !strings.Contains(result, "Testar exportador") {
		t.Errorf("esperava 'Testar exportador' no relatório, mas não foi encontrado")
	}
	if !strings.Contains(result, "[x] Fazer setup") {
		t.Errorf("esperava tarefa concluída, mas não encontrou")
	}
	if !strings.Contains(result, "**Usar Go puro**") {
		t.Errorf("esperava decisão, mas não encontrou")
	}
	if !strings.Contains(result, "50%") {
		t.Errorf("esperava 50%% de progresso, obteve: %s", result)
	}
}

func TestExportHTML(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, ".agents", "session")
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		t.Fatalf("falha ao criar pasta de sessão: %v", err)
	}

	os.WriteFile(filepath.Join(sessionDir, "state.md"), []byte("## Objetivo Atual\n- Test HTML\n## Status em Tempo Real\n- **Fase Atual:** Final"), 0644)
	os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte("- [x] Done\n- [ ] Pending"), 0644)

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	transcriptContent := "{\"type\": \"PLANNER_RESPONSE\", \"thinking\": \"Let's test\", \"created_at\": \"" + time.Now().Format(time.RFC3339) + "\"}\n" +
		"{\"type\": \"PLANNER_RESPONSE\", \"tool_calls\": [{\"name\": \"invoke_subagent\"}], \"created_at\": \"" + time.Now().Add(5*time.Second).Format(time.RFC3339) + "\"}\n"
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	opts := ExportOptions{
		Format:         "html",
		TargetDir:      tempDir,
		TranscriptPath: transcriptPath,
	}

	result, err := Export(opts)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if !strings.Contains(result, "Test HTML") {
		t.Errorf("esperava 'Test HTML' no html, obteve: %s", result)
	}
	if !strings.Contains(result, "Subagentes") || !strings.Contains(result, "1") {
		t.Errorf("esperava métricas de subagentes, não encontrou")
	}
}

func TestParseTranscript_LargeLineAndSubagents(t *testing.T) {
	tempDir := t.TempDir()
	transcriptPath := filepath.Join(tempDir, "large_transcript.jsonl")

	// 1. Line with invoke_subagent
	line1 := `{"type": "PLANNER_RESPONSE", "thinking": "Spawning researcher", "tool_calls": [{"name": "invoke_subagent"}], "created_at": "2026-10-15T12:00:00Z"}` + "\n"

	// 2. Large line (1.5 MB) that would fail standard 1MB bufio.Scanner
	largePayload := strings.Repeat("A", 1500000)
	line2 := `{"type": "USER_INPUT", "content": "` + largePayload + `", "created_at": "2026-10-15T12:01:00Z"}` + "\n"

	// 3. Line with regular tool_call
	line3 := `{"type": "PLANNER_RESPONSE", "tool_calls": [{"name": "run_command"}], "created_at": "2026-10-15T12:02:00Z"}` + "\n"

	err := os.WriteFile(transcriptPath, []byte(line1+line2+line3), 0644)
	if err != nil {
		t.Fatalf("failed to write large transcript: %v", err)
	}

	metrics := parseTranscript(transcriptPath)
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}

	if metrics.SubagentsInvoked != 1 {
		t.Errorf("expected 1 subagent invoked, got %d", metrics.SubagentsInvoked)
	}
	if metrics.ToolCalls != 2 {
		t.Errorf("expected 2 tool calls, got %d", metrics.ToolCalls)
	}
	if metrics.Thoughts != 1 {
		t.Errorf("expected 1 thought, got %d", metrics.Thoughts)
	}
	if metrics.Duration != "2m0s" {
		t.Errorf("expected duration 2m0s, got %s", metrics.Duration)
	}
}
