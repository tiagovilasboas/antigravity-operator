package checkpoint_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagoboas/antigravity-operator/internal/checkpoint"
)

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v, out: %s", strings.Join(args, " "), err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")

	// Criar arquivo inicial e comitar
	initialFile := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(initialFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "initial commit")

	return tempDir
}

func TestCheckpointLifecycle(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// 1. Criar checkpoint em repo limpo
	chk1, err := checkpoint.Create(repoDir, "pre-feature", "antes de começar refactor")
	if err != nil {
		t.Fatalf("Create checkpoint falhou: %v", err)
	}

	if chk1.Name != "pre-feature" {
		t.Errorf("esperava nome 'pre-feature', obteve %s", chk1.Name)
	}
	if chk1.CommitSHA == "" {
		t.Errorf("esperava commit SHA preenchido")
	}

	// 2. Listar checkpoints
	list, err := checkpoint.List(repoDir)
	if err != nil {
		t.Fatalf("List checkpoints falhou: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("esperava 1 checkpoint na lista, obteve %d", len(list))
	}

	// 3. Simular alteração e erro do agente
	mainFile := filepath.Join(repoDir, "main.go")
	_ = os.WriteFile(mainFile, []byte("CÓDIGO QUEBRADO PELO AGENTE COM ERRO DE SINTAXE"), 0644)

	newFile := filepath.Join(repoDir, "broken_file.txt")
	_ = os.WriteFile(newFile, []byte("arquivo lixo criado pelo agente"), 0644)

	// 4. Executar Rollback
	restoredChk, err := checkpoint.Rollback(repoDir, "latest")
	if err != nil {
		t.Fatalf("Rollback falhou: %v", err)
	}

	if restoredChk.ID != chk1.ID {
		t.Errorf("esperava restaurar %s, obteve %s", chk1.ID, restoredChk.ID)
	}

	// 5. Verificar que arquivos foram restaurados com sucesso
	content, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("falha ao ler main.go após rollback: %v", err)
	}
	if !strings.Contains(string(content), "func main()") {
		t.Errorf("main.go não foi revertido ao estado original: %s", string(content))
	}

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Errorf("arquivo lixo untracked deveria ter sido removido no rollback")
	}
}

func TestCheckpoint_NotGitRepo(t *testing.T) {
	nonGit := t.TempDir()
	_, err := checkpoint.Create(nonGit, "test", "desc")
	if err == nil {
		t.Error("esperava erro em diretório que não é git repo")
	}
	_, err = checkpoint.Rollback(nonGit, "latest")
	if err == nil {
		t.Error("esperava erro ao tentar rollback em repositório não git")
	}
}

func TestCheckpoint_WithDirtyFiles(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Cria arquivo dirty
	dirtyFile := filepath.Join(repoDir, "dirty.txt")
	_ = os.WriteFile(dirtyFile, []byte("conteudo inicial dirty"), 0644)

	// Cria checkpoint com dirty files
	chk, err := checkpoint.Create(repoDir, "dirty-checkpoint", "checkpoint com dirty file")
	if err != nil {
		t.Fatalf("falha ao criar checkpoint: %v", err)
	}

	if len(chk.DirtyFiles) == 0 {
		t.Errorf("esperava dirty files registrados no checkpoint")
	}

	// Altera o dirty file
	_ = os.WriteFile(dirtyFile, []byte("conteudo quebrado"), 0644)

	// Executa rollback
	_, err = checkpoint.Rollback(repoDir, chk.ID)
	if err != nil {
		t.Fatalf("falha no rollback: %v", err)
	}

	// Testa erro quando checkpoint não existe
	_, err = checkpoint.Rollback(repoDir, "chk-inexistente")
	if err == nil {
		t.Error("esperava erro ao pedir checkpoint inexistente")
	}
}
