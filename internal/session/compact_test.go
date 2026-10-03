package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

func TestCompact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-compact-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = session.Init(tempDir, false)
	if err != nil {
		t.Fatalf("Init falhou: %v", err)
	}

	todoFile := filepath.Join(tempDir, ".agents", "session", "todo.md")

	// 1. Teste de arquivo sem tarefas suficientes para compactar
	opts := session.DefaultCompactOptions()
	res, err := session.Compact(tempDir, opts)
	if err != nil {
		t.Fatalf("Compact falhou em sessão com poucas tarefas: %v", err)
	}
	if !res.AlreadyCompact {
		t.Errorf("esperava AlreadyCompact=true, obteve false")
	}

	// 2. Preencher todo.md com 10 tarefas concluídas e 3 pendentes
	var lines []string
	lines = append(lines, "# Lista de Tarefas")
	lines = append(lines, "## Pendentes")
	lines = append(lines, "- [ ] Pendente 1")
	lines = append(lines, "- [ ] Pendente 2")
	lines = append(lines, "## Concluídas")
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("- [x] Tarefa finalizada %d", i))
	}
	_ = os.WriteFile(todoFile, []byte(strings.Join(lines, "\n")), 0644)

	// 3. Teste com DryRun=true
	opts.Threshold = 5
	opts.KeepLast = 3
	opts.DryRun = true

	resDry, err := session.Compact(tempDir, opts)
	if err != nil {
		t.Fatalf("Compact DryRun falhou: %v", err)
	}
	if resDry.CompactedTasks != 7 {
		t.Errorf("esperava 7 tarefas compactadas no DryRun, obteve %d", resDry.CompactedTasks)
	}
	if resDry.RetainedTasks != 3 {
		t.Errorf("esperava 3 tarefas retidas, obteve %d", resDry.RetainedTasks)
	}

	// 4. Executar compactação real (DryRun=false)
	opts.DryRun = false
	resReal, err := session.Compact(tempDir, opts)
	if err != nil {
		t.Fatalf("Compact real falhou: %v", err)
	}
	if resReal.CompactedTasks != 7 {
		t.Errorf("esperava 7 tarefas compactadas, obteve %d", resReal.CompactedTasks)
	}

	// Verificar se arquivo de histórico foi criado
	if _, err := os.Stat(resReal.ArchiveFile); os.IsNotExist(err) {
		t.Errorf("arquivo de archive esperado não existe: %s", resReal.ArchiveFile)
	}

	// Verificar se todo.md foi enxugado
	newTodoBytes, err := os.ReadFile(todoFile)
	if err != nil {
		t.Fatalf("falha ao ler novo todo.md: %v", err)
	}
	newTodoStr := string(newTodoBytes)

	if !strings.Contains(newTodoStr, "Histórico compactado: 7 tarefas") {
		t.Errorf("esperava mensagem de histórico compactado no todo.md")
	}
	if strings.Contains(newTodoStr, "- [x] Tarefa finalizada 1\n") {
		t.Errorf("tarefa antiga 1 não deveria estar no todo.md ativo")
	}
	if !strings.Contains(newTodoStr, "- [x] Tarefa finalizada 10") {
		t.Errorf("tarefa recente 10 deveria ser mantida no todo.md")
	}
	if !strings.Contains(newTodoStr, "- [ ] Pendente 1") {
		t.Errorf("tarefas pendentes devem permanecer intactas")
	}
}

// TestCompact_ExceptionsAndEdgeCases valida resiliência contra exceções e cenários extremos.
func TestCompact_ExceptionsAndEdgeCases(t *testing.T) {
	// 1. Exceção: todo.md ausente (diretório inválido ou sem sessão inicializada)
	t.Run("MissingTodoFile", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "compact-missing-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(emptyDir)

		res, err := session.Compact(emptyDir, session.DefaultCompactOptions())
		if err == nil {
			t.Errorf("esperava erro ao tentar compactar com todo.md inexistente, obteve sucesso: %+v", res)
		}
		if !strings.Contains(err.Error(), "não encontrado") {
			t.Errorf("mensagem de erro inesperada: %v", err)
		}
	})

	// 2. Borda: todo.md completamente vazio (0 bytes)
	t.Run("EmptyTodoFile", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-empty-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(""), 0644)

		res, err := session.Compact(tempDir, session.DefaultCompactOptions())
		if err != nil {
			t.Fatalf("Compact em arquivo vazio não deveria falhar: %v", err)
		}
		if !res.AlreadyCompact {
			t.Errorf("esperava AlreadyCompact=true para arquivo vazio")
		}
		if res.CompactedTasks != 0 {
			t.Errorf("esperava 0 tarefas compactadas, obteve %d", res.CompactedTasks)
		}
	})

	// 3. Borda: todo.md contendo apenas tarefas pendentes (- [ ])
	t.Run("OnlyPendingTasks", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-pending-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		content := "# Tarefas\n- [ ] Tarefa 1\n- [ ] Tarefa 2\n- [ ] Tarefa 3\n- [ ] Tarefa 4\n- [ ] Tarefa 5\n- [ ] Tarefa 6\n"
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(content), 0644)

		res, err := session.Compact(tempDir, session.CompactOptions{Threshold: 2, KeepLast: 1})
		if err != nil {
			t.Fatalf("Compact falhou: %v", err)
		}
		if !res.AlreadyCompact {
			t.Errorf("esperava AlreadyCompact=true quando não há tarefas concluídas")
		}
		if res.CompactedTasks != 0 {
			t.Errorf("esperava 0 tarefas compactadas, obteve %d", res.CompactedTasks)
		}
	})

	// 4. Parâmetros inválidos: Threshold negativo e KeepLast negativo
	t.Run("NegativeOptionsNormalization", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-negative-opts-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		var lines []string
		for i := 1; i <= 8; i++ {
			lines = append(lines, fmt.Sprintf("- [x] Tarefa %d", i))
		}
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(strings.Join(lines, "\n")), 0644)

		// Threshold -5 vira 5, KeepLast -3 vira 0 (arquiva tudo)
		res, err := session.Compact(tempDir, session.CompactOptions{Threshold: -5, KeepLast: -3})
		if err != nil {
			t.Fatalf("Compact com opções negativas não deve falhar: %v", err)
		}
		if res.CompactedTasks != 8 {
			t.Errorf("com KeepLast normalizado para 0, esperava 8 tarefas compactadas, obteve %d", res.CompactedTasks)
		}
		if res.RetainedTasks != 0 {
			t.Errorf("esperava 0 tarefas retidas, obteve %d", res.RetainedTasks)
		}
	})

	// 5. KeepLast maior que o total de tarefas concluídas
	t.Run("KeepLastGreaterThanTotal", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-keeplast-high-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		var lines []string
		for i := 1; i <= 6; i++ {
			lines = append(lines, fmt.Sprintf("- [x] Tarefa %d", i))
		}
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(strings.Join(lines, "\n")), 0644)

		res, err := session.Compact(tempDir, session.CompactOptions{Threshold: 3, KeepLast: 10})
		if err != nil {
			t.Fatalf("Compact falhou: %v", err)
		}
		if !res.AlreadyCompact {
			t.Errorf("esperava AlreadyCompact=true pois KeepLast(10) > Total(6)")
		}
		if res.CompactedTasks != 0 {
			t.Errorf("esperava 0 tarefas compactadas, obteve %d", res.CompactedTasks)
		}
	})

	// 6. Idempotência / Execuções consecutivas
	t.Run("SequentialCompacts", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-sequential-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		var lines []string
		for i := 1; i <= 10; i++ {
			lines = append(lines, fmt.Sprintf("- [x] Tarefa %d", i))
		}
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(strings.Join(lines, "\n")), 0644)

		opts := session.CompactOptions{Threshold: 5, KeepLast: 3}

		// Primeiro ciclo
		res1, err := session.Compact(tempDir, opts)
		if err != nil {
			t.Fatalf("primeiro Compact falhou: %v", err)
		}
		if res1.CompactedTasks != 7 {
			t.Errorf("primeiro compact esperava 7 tarefas, obteve %d", res1.CompactedTasks)
		}

		// Segundo ciclo imediato
		res2, err := session.Compact(tempDir, opts)
		if err != nil {
			t.Fatalf("segundo Compact falhou: %v", err)
		}
		if !res2.AlreadyCompact {
			t.Errorf("segundo compact deveria retornar AlreadyCompact=true")
		}
		if res2.CompactedTasks != 0 {
			t.Errorf("segundo compact esperava 0 tarefas compactadas, obteve %d", res2.CompactedTasks)
		}
	})

	// 7. Exceção de E/S: Falha ao criar diretório de archive
	t.Run("ArchiveDirectoryCreationFailure", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-io-err-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)
		var lines []string
		for i := 1; i <= 10; i++ {
			lines = append(lines, fmt.Sprintf("- [x] Tarefa %d", i))
		}
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte(strings.Join(lines, "\n")), 0644)

		// Criar um arquivo comum com o mesmo nome da pasta "archive" para forçar erro no os.MkdirAll
		blockedArchive := filepath.Join(sessionDir, "archive")
		_ = os.WriteFile(blockedArchive, []byte("bloqueio"), 0644)

		res, err := session.Compact(tempDir, session.CompactOptions{Threshold: 5, KeepLast: 3})
		if err == nil {
			t.Errorf("esperava erro de I/O ao tentar criar pasta archive sobre arquivo existente, obteve sucesso: %+v", res)
		}
		if !strings.Contains(err.Error(), "pasta de arquivo") {
			t.Errorf("mensagem de erro inesperada: %v", err)
		}
	})

	// 8. Compactação de seções históricas no state.md
	t.Run("StateHistoricalSectionsCompaction", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "compact-state-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		sessionDir := filepath.Join(tempDir, ".agents", "session")
		_ = os.MkdirAll(sessionDir, 0755)

		// todo.md já compacto (2 tarefas)
		_ = os.WriteFile(filepath.Join(sessionDir, "todo.md"), []byte("# Tarefas\n- [x] T1\n- [x] T2\n"), 0644)

		// state.md com seções históricas acumuladas
		stateContent := `# Estado da Sessão
## Objetivo Atual
- Entregar feature X
## Status em Tempo Real
- Fase: Execução
## Histórico de Execuções
- Turno 1: tentou X e falhou
- Turno 2: corrigiu Y
- Turno 3: deploy efetuado
## Próximos Passos Imediatos
1. Passo final
`
		statePath := filepath.Join(sessionDir, "state.md")
		_ = os.WriteFile(statePath, []byte(stateContent), 0644)

		res, err := session.Compact(tempDir, session.DefaultCompactOptions())
		if err != nil {
			t.Fatalf("Compact de state falhou: %v", err)
		}

		if !res.StateCompacted {
			t.Errorf("esperava StateCompacted=true")
		}
		if res.StateBytesSaved <= 0 {
			t.Errorf("esperava StateBytesSaved > 0, obteve %d", res.StateBytesSaved)
		}

		// Validar que o state.md ativo não contém mais a seção de histórico
		cleanStateBytes, _ := os.ReadFile(statePath)
		cleanStateStr := string(cleanStateBytes)
		if strings.Contains(cleanStateStr, "Histórico de Execuções") {
			t.Errorf("state.md limpo ainda contém seção de histórico")
		}
		if !strings.Contains(cleanStateStr, "Entregar feature X") {
			t.Errorf("state.md limpo deve manter o Objetivo Atual")
		}
		if !strings.Contains(cleanStateStr, "Passo final") {
			t.Errorf("state.md limpo deve manter os Próximos Passos")
		}

		// Validar que o arquivo de histórico foi criado em archive
		if res.StateArchiveFile == "" {
			t.Errorf("esperava StateArchiveFile preenchido")
		}
		if _, err := os.Stat(res.StateArchiveFile); os.IsNotExist(err) {
			t.Errorf("arquivo de arquivo de estado não existe: %s", res.StateArchiveFile)
		}
	})
}
