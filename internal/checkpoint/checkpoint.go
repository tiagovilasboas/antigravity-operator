package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Checkpoint representa um instantâneo atômico de segurança do workspace.
type Checkpoint struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Timestamp   string   `json:"timestamp"`
	CommitSHA   string   `json:"commit_sha"`
	Branch      string   `json:"branch"`
	DirtyFiles  []string `json:"dirty_files"`
	StashSHA    string   `json:"stash_sha,omitempty"`
	Description string   `json:"description"`
	// Untracked lista os arquivos não rastreados no momento do checkpoint.
	// O rollback nunca os apaga (o stash não guarda o conteúdo deles).
	Untracked []string `json:"untracked,omitempty"`
}

// Create cria um checkpoint de segurança registrando o estado exato do git e arquivos modificados.
func Create(targetDir string, name string, description string) (*Checkpoint, error) {
	if !isGitRepo(targetDir) {
		return nil, fmt.Errorf("diretório %s não é um repositório git (necessário para checkpoints atômicos)", targetDir)
	}

	if runGit(targetDir, "rev-parse", "--verify", "-q", "HEAD") == "" {
		return nil, fmt.Errorf("o repositório em %s ainda não tem commits; faça o primeiro commit antes de criar um checkpoint", targetDir)
	}

	branch := runGit(targetDir, "rev-parse", "--abbrev-ref", "HEAD")
	commit := runGit(targetDir, "rev-parse", "--short", "HEAD")
	statusOut := runGit(targetDir, "status", "--porcelain")

	var dirtyFiles []string
	if statusOut != "" {
		lines := strings.Split(statusOut, "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				dirtyFiles = append(dirtyFiles, l)
			}
		}
	}

	now := time.Now()
	id := fmt.Sprintf("chk-%s", now.Format("20060102-150405"))
	if name == "" {
		name = fmt.Sprintf("checkpoint-%s", now.Format("150405"))
	}

	chk := &Checkpoint{
		ID:          id,
		Name:        name,
		Timestamp:   now.Format(time.RFC3339),
		CommitSHA:   commit,
		Branch:      branch,
		DirtyFiles:  dirtyFiles,
		Description: description,
		Untracked:   untrackedFiles(targetDir),
	}

	// Se houver arquivos modificados, cria um objeto de stash commit sem alterar a working tree
	if len(dirtyFiles) > 0 {
		stashSHA := runGit(targetDir, "stash", "create", fmt.Sprintf("agyo-checkpoint: %s", name))
		chk.StashSHA = strings.TrimSpace(stashSHA)
	}

	// Persiste o checkpoint no arquivo de controle da sessão
	sessionDir := filepath.Join(targetDir, ".agents", "session")
	_ = os.MkdirAll(sessionDir, 0755)
	chkFile := filepath.Join(sessionDir, "checkpoints.json")

	list, _ := List(targetDir)
	list = append(list, *chk)

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar checkpoints: %w", err)
	}

	if err := os.WriteFile(chkFile, data, 0644); err != nil {
		return nil, fmt.Errorf("falha ao salvar %s: %w", chkFile, err)
	}

	return chk, nil
}

// List retorna a lista de todos os checkpoints salvos, do mais recente para o mais antigo.
func List(targetDir string) ([]Checkpoint, error) {
	chkFile := filepath.Join(targetDir, ".agents", "session", "checkpoints.json")
	if _, err := os.Stat(chkFile); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(chkFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler %s: %w", chkFile, err)
	}

	var list []Checkpoint
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("falha ao decodificar %s: %w", chkFile, err)
	}

	// Ordena do mais recente para o mais antigo
	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp > list[j].Timestamp
	})

	return list, nil
}

// Rollback restaura o workspace para o estado exato gravado em um checkpoint.
func Rollback(targetDir string, targetIDOrName string) (*Checkpoint, error) {
	if !isGitRepo(targetDir) {
		return nil, fmt.Errorf("diretório %s não é um repositório git", targetDir)
	}

	list, err := List(targetDir)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("nenhum checkpoint encontrado para rollback em %s", targetDir)
	}

	var targetChk *Checkpoint
	if targetIDOrName == "" || targetIDOrName == "latest" {
		targetChk = &list[0]
	} else {
		for i := range list {
			if list[i].ID == targetIDOrName || list[i].Name == targetIDOrName {
				targetChk = &list[i]
				break
			}
		}
	}

	if targetChk == nil {
		return nil, fmt.Errorf("checkpoint '%s' não encontrado", targetIDOrName)
	}

	// 1. Limpa alterações não commitadas locais
	if err := execGit(targetDir, "reset", "--hard", "HEAD"); err != nil {
		return nil, fmt.Errorf("falha ao descartar alterações locais (git reset --hard HEAD): %w", err)
	}
	// Remove só arquivos criados depois do checkpoint. Nunca apaga .agents/
	// (memória da sessão e checkpoints.json) nem arquivos que já existiam
	// sem rastreio no checkpoint, cujo conteúdo o stash não guarda.
	keep := make(map[string]bool, len(targetChk.Untracked))
	for _, f := range targetChk.Untracked {
		keep[f] = true
	}
	for _, f := range untrackedFiles(targetDir) {
		if !keep[f] {
			_ = os.Remove(filepath.Join(targetDir, f))
		}
	}

	// 2. Se o checkpoint tinha stash registrado, reaplica
	if targetChk.StashSHA != "" {
		if err := execGit(targetDir, "stash", "apply", targetChk.StashSHA); err != nil {
			// Se stash falhar (ex: conflito de index), restaura ao commit base
			_ = execGit(targetDir, "reset", "--hard", targetChk.CommitSHA)
			return nil, fmt.Errorf("falha ao reaplicar as alterações do checkpoint (stash %s); working tree restaurada ao commit %s: %w", targetChk.StashSHA, targetChk.CommitSHA, err)
		}
	} else if targetChk.CommitSHA != "" && runGit(targetDir, "rev-parse", targetChk.CommitSHA+"^{commit}") != runGit(targetDir, "rev-parse", "HEAD") {
		// Só troca de commit se HEAD andou; senão o checkout destacaria o HEAD da branch.
		_ = execGit(targetDir, "checkout", targetChk.CommitSHA)
	}

	// 3. Atualiza nota de rollback no state.md se presente
	statePath := filepath.Join(targetDir, ".agents", "session", "state.md")
	if stateBytes, err := os.ReadFile(statePath); err == nil {
		note := fmt.Sprintf("\n> ⚠️ *Rollback efetuado para o checkpoint '%s' (%s) em %s*\n",
			targetChk.Name, targetChk.ID, time.Now().Format("2006-01-02 15:04:05"))
		_ = os.WriteFile(statePath, []byte(string(stateBytes)+note), 0644)
	}

	return targetChk, nil
}

// untrackedFiles lista arquivos não rastreados e não ignorados, relativos a
// dir, excluindo .agents/ (memória da sessão do próprio agyo).
func untrackedFiles(dir string) []string {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude).agents")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func runGit(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func execGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
