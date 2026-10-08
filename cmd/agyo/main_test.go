package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestCLI_PrintUsage(t *testing.T) {
	printUsage()
}

func TestCLI_PrintAbout(t *testing.T) {
	printAbout()
}

func TestCLI_RunInitAndSession(t *testing.T) {
	targetDir := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: t.TempDir(),
	}

	// 1. Initial init
	runInit([]string{targetDir})

	// 2. Force init
	runInit([]string{"--force", targetDir})

	// 3. Session status (standard and JSON)
	runSession(info, []string{"status", targetDir})
	runSession(info, []string{"status", "--json", targetDir})

	// 4. Session archive
	runSession(info, []string{"archive", targetDir})

	// 5. Session watch with prepared brain log (Antigravity 2.0 app data dir)
	info.HomeDir = t.TempDir()
	brainDir := filepath.Join(info.HomeDir, ".gemini", "antigravity", "brain", "conv-test", ".system_generated", "logs")
	_ = os.MkdirAll(brainDir, 0755)
	_ = os.WriteFile(filepath.Join(brainDir, "transcript.jsonl"), []byte(`{"step_index":1,"type":"USER_INPUT","content":"Hi"}`+"\n"), 0644)
	runSession(info, []string{"watch", "--once", "--steps", "1"})
}

func TestCLI_RunDoctor(t *testing.T) {
	info, err := platform.Detect()
	if err != nil {
		t.Fatalf("failed to detect platform: %v", err)
	}
	targetDir := t.TempDir()

	// 1. Standard doctor
	runDoctor(info, []string{})

	// 2. Doctor JSON
	runDoctor(info, []string{"--json"})

	// 3. Doctor Fix
	runDoctor(info, []string{"--fix", targetDir})

	// 4. Doctor Fix JSON
	runDoctor(info, []string{"--fix", "--json", targetDir})
}

func TestCLI_RunSync(t *testing.T) {
	tempGemini := filepath.Join(t.TempDir(), ".gemini", "antigravity")
	info := &platform.Info{
		GeminiDir: tempGemini,
	}
	runSync(info, nil)

	// --update-mcp backs up a legacy manifest and rewrites it.
	mcp := filepath.Join(tempGemini, "mcp", "default-servers.json")
	legacy := `{"mcpServers":{"playwright":{"command":"npx","args":["-y","@executeautomation/playwright-mcp-server"]}}}`
	_ = os.WriteFile(mcp, []byte(legacy), 0644)
	runSync(info, []string{"--update-mcp"})
	backups, _ := filepath.Glob(mcp + ".bak-*")
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %v", backups)
	}
	if cur, _ := os.ReadFile(mcp); strings.Contains(string(cur), "executeautomation") {
		t.Fatalf("manifest not rewritten: %s", cur)
	}
}

func TestCLI_RunBrowserCommands(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}
	// Status
	runBrowser(info, []string{"status"})
}

func TestCLI_RunHook(t *testing.T) {
	tempDir := t.TempDir()
	gitHooksDir := filepath.Join(tempDir, ".git", "hooks")
	_ = os.MkdirAll(gitHooksDir, 0755)

	runHook([]string{"install", tempDir})
	runHook([]string{"uninstall", tempDir})
}

func TestCLI_RunSessionExport(t *testing.T) {
	targetDir := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: t.TempDir(),
	}

	runInit([]string{targetDir})

	// Export to stdout
	runSession(info, []string{"export", "--format=markdown", targetDir})

	// Export to file
	outFile := filepath.Join(targetDir, "report.html")
	runSession(info, []string{"export", "--format=html", "--out=" + outFile, targetDir})

	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("arquivo de relatório não gerado: %v", err)
	}
}

func TestCLI_RunWatchTree(t *testing.T) {
	info := &platform.Info{
		OS:      "darwin",
		HomeDir: t.TempDir(),
	}

	// CLI app data dir: the CLI must find it too, not only ~/.gemini/antigravity.
	brainDir := filepath.Join(info.HomeDir, ".gemini", "antigravity-cli", "brain", "conv-test", ".system_generated", "logs")
	_ = os.MkdirAll(brainDir, 0755)
	_ = os.WriteFile(filepath.Join(brainDir, "transcript.jsonl"), []byte(`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"function":{"name":"invoke_subagent","arguments":"{\"Subagents\":[{\"Role\":\"Tester\",\"TypeName\":\"research\",\"Prompt\":\"Check tests\"}]}"}}]}`+"\n"), 0644)

	runSession(info, []string{"watch", "--once", "--steps=1", "--tree"})
}

func TestCLI_RunCompletion(t *testing.T) {
	runCompletion([]string{"bash"})
	runCompletion([]string{"zsh"})
	runCompletion([]string{"fish"})
}

func TestCLI_CheckpointAndRollback(t *testing.T) {
	tempDir := t.TempDir()
	// Inicializa git repo para os testes de checkpoint
	execGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		_ = cmd.Run()
	}
	execGit("init")
	execGit("config", "user.email", "test@test.com")
	execGit("config", "user.name", "Test User")

	_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("initial"), 0644)
	execGit("add", "file.txt")
	execGit("commit", "-m", "initial commit")

	// 1. Checkpoint com working tree limpa
	runCheckpoint([]string{"--desc=Clean Tree Checkpoint", tempDir})

	// 2. Modifica arquivo
	_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("dirty content"), 0644)
	runCheckpoint([]string{"--desc=Dirty File Checkpoint", tempDir})

	// 3. Listar checkpoints via CLI (text e JSON)
	runCheckpoint([]string{"--list", tempDir})
	runCheckpoint([]string{"--list", "--json", tempDir})

	// 4. Modifica mais uma vez e faz rollback via CLI
	_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("bad refactor"), 0644)
	runRollback([]string{"latest", tempDir})
}

func TestCLI_SessionListAndRestore(t *testing.T) {
	targetDir := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: t.TempDir(),
	}

	runInit([]string{targetDir})

	// Arquiva sessão
	runSession(info, []string{"archive", targetDir})

	// Lista sessões (text e JSON)
	runSession(info, []string{"list", targetDir})
	runSession(info, []string{"list", "--json", targetDir})

	// Restaura sessão
	runSession(info, []string{"restore", "latest", targetDir})
}

func TestCLI_RunNotebookCommands(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}
	// 1. Status
	runNotebook(info, []string{"status"})

	// 2. Sem argumentos (usage)
	runNotebook(info, []string{})

	// 3. Subcomando desconhecido (não deve dar panic)
	// captura erro silenciosamente testando usage
}

func TestSubcommandUsageCompleteness(t *testing.T) {
	sessionTests := []struct {
		subcommand string
	}{
		{"status"},
		{"compact"},
		{"archive"},
		{"list"},
		{"restore"},
		{"watch"},
		{"export"},
	}

	sessionUsage := sessionUsageString()
	for _, tc := range sessionTests {
		t.Run("session_"+tc.subcommand, func(t *testing.T) {
			if !strings.Contains(sessionUsage, tc.subcommand) {
				t.Errorf("sessionUsageString() %q does not contain subcommand %q", sessionUsage, tc.subcommand)
			}
		})
	}

	browserTests := []struct {
		subcommand string
	}{
		{"start"},
		{"status"},
		{"stop"},
		{"tabs"},
		{"open"},
		{"close"},
		{"eval"},
		{"shot"},
	}

	browserUsage := browserUsageString()
	for _, tc := range browserTests {
		t.Run("browser_"+tc.subcommand, func(t *testing.T) {
			if !strings.Contains(browserUsage, tc.subcommand) {
				t.Errorf("browserUsageString() %q does not contain subcommand %q", browserUsage, tc.subcommand)
			}
		})
	}
}
