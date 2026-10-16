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
