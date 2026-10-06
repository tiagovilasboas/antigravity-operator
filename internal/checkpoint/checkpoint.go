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
	list, _ := List(targetDir)
	id := uniqueCheckpointID(list, now)
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

// uniqueCheckpointID builds chk-YYYYMMDD-HHMMSS, appending -2, -3, ... when that
// second already has an ID in the existing list so rapid creates stay distinct.
func uniqueCheckpointID(existing []Checkpoint, now time.Time) string {
	base := fmt.Sprintf("chk-%s", now.Format("20060102-150405"))
	used := make(map[string]struct{}, len(existing))
	for _, chk := range existing {
		used[chk.ID] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
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

// RollbackResult descreve o rollback executado.
type RollbackResult struct {
	Checkpoint
	// MovedFrom é o commit em que a branch estava antes do rollback, quando
	// HEAD tinha andado desde o checkpoint ("" quando não andou).
	MovedFrom string
	// MovedBranch é a branch movida de volta ao commit do checkpoint.
	MovedBranch string
	// BackupStash guarda as alterações rastreadas descartadas pelo rollback.
	BackupStash string
}

// Warning explains how to undo a rollback that moved the branch or discarded
// local changes. It is empty when there is nothing to undo.
func (r *RollbackResult) Warning() string {
	var b strings.Builder
	if r.MovedFrom != "" {
		fmt.Fprintf(&b, "⚠️  Branch %s moved from %s back to checkpoint commit %s.\n", r.MovedBranch, shortSHA(r.MovedFrom), r.CommitSHA)
		fmt.Fprintf(&b, "   Undo: git reset --hard %s  (the old commit is also in: git reflog)\n", r.MovedFrom)
	}
	if r.BackupStash != "" {
		where := ""
		if r.MovedFrom != "" {
			where = " (after the undo above; they were made on top of " + shortSHA(r.MovedFrom) + ")"
		}
		fmt.Fprintf(&b, "⚠️  Uncommitted changes were saved before the rollback. Restore: git stash apply %s%s\n", r.BackupStash, where)
	}
	return b.String()
}

// Rollback restaura o workspace para o estado exato gravado em um checkpoint.
func Rollback(targetDir string, targetIDOrName string) (*RollbackResult, error) {
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

	target := runGit(targetDir, "rev-parse", "--verify", "-q", targetChk.CommitSHA+"^{commit}")
	if target == "" {
		return nil, fmt.Errorf("commit %q do checkpoint '%s' não existe mais neste repositório", targetChk.CommitSHA, targetChk.Name)
	}
	head := runGit(targetDir, "rev-parse", "HEAD")
	res := &RollbackResult{Checkpoint: *targetChk}

	// HEAD andou desde o checkpoint: move a branch atual de volta ao commit do
	// checkpoint (git reset --hard), sem destacar o HEAD. Recusa antes de mexer
	// em qualquer coisa se o HEAD já está destacado ou se a branch não é a do
	// checkpoint.
	if target != head {
		branch := runGit(targetDir, "symbolic-ref", "-q", "--short", "HEAD")
		if branch == "" {
			return nil, fmt.Errorf("HEAD está destacado (detached) em %s; faça checkout de uma branch antes do rollback para não perder a referência", shortSHA(head))
		}
		if targetChk.Branch != "" && targetChk.Branch != "HEAD" && targetChk.Branch != branch {
			return nil, fmt.Errorf("o checkpoint '%s' foi criado na branch %s, mas a branch atual é %s; faça checkout de %s antes do rollback", targetChk.Name, targetChk.Branch, branch, targetChk.Branch)
		}
		res.MovedFrom = head
		res.MovedBranch = branch
	}

	// Guarda alterações rastreadas não commitadas num stash recuperável antes
	// de descartá-las (git stash list / git stash apply <sha>).
	if runGit(targetDir, "status", "--porcelain", "--untracked-files=no") != "" {
		msg := fmt.Sprintf("agyo-rollback: backup before %s", targetChk.ID)
		if sha := runGit(targetDir, "stash", "create", msg); sha != "" {
			if err := execGit(targetDir, "stash", "store", "-m", msg, sha); err != nil {
				return nil, fmt.Errorf("falha ao guardar backup das alterações locais (git stash store): %w", err)
			}
			res.BackupStash = sha
		}
	}

	// .agents/ (memória da sessão e checkpoints.json) nunca muda com o rollback,
	// mesmo se estiver versionado e o commit antigo tiver outra versão.
	agents := snapshotDir(filepath.Join(targetDir, ".agents"))

	// 1. Volta a working tree (e a branch, se HEAD andou) ao commit do checkpoint
	if err := execGit(targetDir, "reset", "--hard", target); err != nil {
		restoreDir(agents)
		return nil, fmt.Errorf("falha ao restaurar o commit %s (git reset --hard): %w", shortSHA(target), err)
	}
	// Remove só arquivos criados depois do checkpoint. Nunca apaga .agents/
	// nem arquivos que já existiam sem rastreio no checkpoint, cujo conteúdo o
	// stash não guarda.
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
			_ = execGit(targetDir, "reset", "--hard", target)
			restoreDir(agents)
			return nil, fmt.Errorf("falha ao reaplicar as alterações do checkpoint (stash %s); working tree restaurada ao commit %s: %w", targetChk.StashSHA, targetChk.CommitSHA, err)
		}
	}
	restoreDir(agents)

	// 3. Atualiza nota de rollback no state.md se presente
	statePath := filepath.Join(targetDir, ".agents", "session", "state.md")
	if stateBytes, err := os.ReadFile(statePath); err == nil {
		note := fmt.Sprintf("\n> ⚠️ *Rollback efetuado para o checkpoint '%s' (%s) em %s*\n",
			targetChk.Name, targetChk.ID, time.Now().Format("2006-01-02 15:04:05"))
		_ = os.WriteFile(statePath, []byte(string(stateBytes)+note), 0644)
	}

	return res, nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// snapshotDir lê todos os arquivos regulares sob dir.
func snapshotDir(dir string) map[string][]byte {
	files := map[string][]byte{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if b, err := os.ReadFile(p); err == nil {
				files[p] = b
			}
		}
		return nil
	})
	return files
}

// restoreDir regrava os arquivos de um snapshot (sem apagar nada).
func restoreDir(files map[string][]byte) {
	for p, b := range files {
		if cur, err := os.ReadFile(p); err == nil && string(cur) == string(b) {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, b, 0o644)
	}
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
