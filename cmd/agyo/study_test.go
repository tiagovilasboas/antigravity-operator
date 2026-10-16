package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestStudy_Slugify(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Calculo III - Vetores & Integrais", "calculo-iii-vetores-integrais"},
		{"Estruturas de Dados em Go!", "estruturas-de-dados-em-go"},
		{"", ""},
		{"   Deep   Learning   ", "deep-learning"},
	}

	for _, c := range cases {
		got := slugify(c.input)
		if got != c.expected {
			t.Errorf("slugify(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestStudy_GenerateFlashcards(t *testing.T) {
	sampleText := `Aqui esta o resumo:
Pergunta: O que e a Lei de Amdahl?
Resposta: E uma formula que calcula o ganho teorico maximo de desempenho com paralelismo.
Q: O que e latencia?
A: O tempo decorrido entre a requisicao e a primeira resposta.`

	cards := generateFlashcards(sampleText)
	if !strings.Contains(cards, "#flashcard") {
		t.Errorf("expected flashcard tag in output: %s", cards)
	}
	if !strings.Contains(cards, "Lei de Amdahl") {
		t.Errorf("expected Lei de Amdahl question: %s", cards)
	}
	if !strings.Contains(cards, "latencia") {
		t.Errorf("expected latencia question: %s", cards)
	}
}

func TestStudy_GenerateFlashcards_Fallback(t *testing.T) {
	rawText := "Nenhum formato explicito de pergunta aqui, apenas texto corrido sobre compiladores e analise sintatica."
	cards := generateFlashcards(rawText)
	if !strings.Contains(cards, "#flashcard") {
		t.Errorf("expected flashcard fallback tag: %s", cards)
	}
	if !strings.Contains(cards, "compiladores") {
		t.Errorf("expected content in fallback card: %s", cards)
	}
}

func TestStudy_Usage(t *testing.T) {
	info := &platform.Info{}
	// Calling with empty args shouldn't crash
	runStudy(info, []string{})
	runStudy(info, []string{"--help"})
}

func TestStudy_ExamPrep_PlanGeneration(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	// Prepara estrutura .agents/session/todo.md
	sessionDir := filepath.Join(".agents", "session")
	_ = os.MkdirAll(sessionDir, 0755)
	todoPath := filepath.Join(sessionDir, "todo.md")
	_ = os.WriteFile(todoPath, []byte("# Tarefas Ativas\n"), 0644)

	info := &platform.Info{
		GeminiDir: tempDir,
	}

	runStudy(info, []string{"exam-prep", "--disciplina", "Sistemas Distribuidos", "--data", "2026-11-30", "--dias", "7"})

	updatedTodo, err := os.ReadFile(todoPath)
	if err != nil {
		t.Fatalf("failed to read updated todo.md: %v", err)
	}

	content := string(updatedTodo)
	if !strings.Contains(content, "Cronograma de Prova: Sistemas Distribuidos") {
		t.Errorf("expected schedule header in todo.md, got: %s", content)
	}
	if !strings.Contains(content, "Fundamentos e Leitura Base") {
		t.Errorf("expected milestone 1 in todo.md, got: %s", content)
	}
	if !strings.Contains(content, "Dia da Prova") {
		t.Errorf("expected exam day milestone in todo.md, got: %s", content)
	}
}

func TestStudy_Quiz_ExitGracefully(t *testing.T) {
	info := &platform.Info{}
	oldStdin := studyStdin
	defer func() { studyStdin = oldStdin }()

	// Simula usuário digitando "sair"
	studyStdin = bytes.NewBufferString("sair\n")

	// Usando porta inativa: AskNotebook falhará na primeira chamada mas não deve entrar em pânico
	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	runStudy(info, []string{"quiz", "--notebook", "test-nb", "--port", "59994"})
	if !exitCalled {
		t.Logf("quiz exited without error code")
	}
}

func TestStudy_Ingest_Validation(t *testing.T) {
	info := &platform.Info{}
	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()

	// 1. Missing positional target
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	runStudy(info, []string{"ingest"})
	if !exitCalled {
		t.Errorf("expected exit on missing target")
	}

	// 2. Missing notebook flag
	exitCalled = false
	runStudy(info, []string{"ingest", "https://youtube.com/watch?v=123"})
	if !exitCalled {
		t.Errorf("expected exit on missing --notebook flag")
	}
}

func TestStudy_ExamPrep_SanitizesNewlines(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	sessionDir := filepath.Join(".agents", "session")
	_ = os.MkdirAll(sessionDir, 0755)
	todoPath := filepath.Join(sessionDir, "todo.md")
	_ = os.WriteFile(todoPath, []byte("# Tarefas\n"), 0644)

	info := &platform.Info{GeminiDir: tempDir}

	// Injeta quebras de linha no nome da disciplina
	runStudy(info, []string{"exam-prep", "--disciplina", "Seguranca\nInjetada\r\nTask", "--data", "2026-12-01", "--context", "estudos"})

	updatedTodo, err := os.ReadFile(todoPath)
	if err != nil {
		t.Fatalf("failed to read todo: %v", err)
	}

	content := string(updatedTodo)
	if strings.Contains(content, "Seguranca\nInjetada") {
		t.Errorf("expected newlines to be stripped from disciplina: %s", content)
	}
	if !strings.Contains(content, "Seguranca Injetada Task") {
		t.Errorf("expected sanitized string 'Seguranca Injetada Task': %s", content)
	}
}

func TestStudy_Ingest_PermuteArgsFlagsAfterPositional(t *testing.T) {
	info := &platform.Info{}
	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()

	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
	}

	// Quando as flags vem apos a URL, permuteArgs deve posicionar as flags antes do positional
	// garantindo que --notebook seja reconhecido. Na ausencia de porta DevTools ativa, o comando
	// deve falhar no runtime (ExitRuntimeError = 1), e NAO na validacao de uso (ExitUsageError = 2).
	runStudy(info, []string{"ingest", "https://youtube.com/watch?v=123", "--notebook", "nb-test", "--port", "59994"})
	if exitCode == ExitUsageError {
		t.Fatalf("expected --notebook to be recognized after positional argument, but failed with ExitUsageError")
	}
	if exitCode != ExitRuntimeError {
		t.Fatalf("expected ExitRuntimeError (due to inactive DevTools port), got %d", exitCode)
	}
}

func TestPermuteArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		boolFlags map[string]bool
		expected  []string
	}{
		{
			name:      "flags after positional",
			args:      []string{"https://youtube.com/watch?v=123", "--notebook", "nb-test", "--port", "59994"},
			boolFlags: nil,
			expected:  []string{"--notebook", "nb-test", "--port", "59994", "https://youtube.com/watch?v=123"},
		},
		{
			name:      "boolean flag after positional",
			args:      []string{"doc-123", "--json"},
			boolFlags: map[string]bool{"json": true},
			expected:  []string{"--json", "doc-123"},
		},
		{
			name:      "flag with equals",
			args:      []string{"doc-123", "--out=file.txt"},
			boolFlags: nil,
			expected:  []string{"--out=file.txt", "doc-123"},
		},
		{
			name:      "flags already before positional",
			args:      []string{"--notebook", "nb-test", "https://youtube.com/watch?v=123"},
			boolFlags: nil,
			expected:  []string{"--notebook", "nb-test", "https://youtube.com/watch?v=123"},
		},
		{
			name:      "end of flags delimiter",
			args:      []string{"pos1", "--", "pos2", "--notebook", "nb-test"},
			boolFlags: nil,
			expected:  []string{"pos1", "--", "pos2", "--notebook", "nb-test"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := permuteArgs(tc.args, tc.boolFlags)
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tc.expected[i], got[i])
				}
			}
		})
	}
}
