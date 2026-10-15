package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

	// Antigravity lifecycle hooks via CLI
	runHook([]string{"antigravity", "status", "--scope=workspace", tempDir})
	runHook([]string{"antigravity", "install", "--scope=workspace", "--dry-run", tempDir})
	runHook([]string{"antigravity", "install", "--scope=workspace", tempDir})
	runHook([]string{"antigravity", "status", "--scope=workspace", tempDir})
	runHook([]string{"antigravity", "uninstall", "--scope=workspace", tempDir})

	// Run events via CLI
	runHook([]string{"run", "PreInvocation"})
	runHook([]string{"run", "PostToolUse"})
	runHook([]string{"run", "Stop"})
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

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var lastCode int
	exitFunc = func(code int) {
		lastCode = code
	}

	// 1. Status (port 9222 offline)
	runNotebook(info, []string{"status"})

	// 2. Sem argumentos (usage -> exit 2)
	lastCode = -1
	runNotebook(info, []string{})
	if lastCode != ExitUsageError {
		t.Errorf("expected exit code %d for empty notebook command, got %d", ExitUsageError, lastCode)
	}

	// 3. Subcomando desconhecido -> exit 2
	lastCode = -1
	runNotebook(info, []string{"bogus"})
	if lastCode != ExitUsageError {
		t.Errorf("expected exit code %d for unknown notebook subcommand, got %d", ExitUsageError, lastCode)
	}
}

func TestCLI_ExitCodeContract_HelpFlags(t *testing.T) {
	// UX-1 & UX-2: --help, -h, help must return 0 and have NO side effects (no Chrome start, no files written)
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var lastCode int
	exitFunc = func(code int) {
		lastCode = code
	}

	groups := [][]string{
		{"session", "--help"},
		{"session", "-h"},
		{"session", "help"},
		{"browser", "--help"},
		{"browser", "-h"},
		{"browser", "help"},
		{"hook", "--help"},
		{"hook", "-h"},
		{"hook", "help"},
		{"checkpoint", "--help"},
		{"checkpoint", "-h"},
		{"checkpoint", "help"},
		{"notebooklm", "--help"},
		{"notebooklm", "-h"},
		{"notebooklm", "help"},
		// UX-1: notebook open --help must NOT start Chrome
		{"notebooklm", "open", "--help"},
		{"notebooklm", "open", "-h"},
		{"notebooklm", "status", "--help"},
		{"browser", "open", "--help"},
		{"browser", "shot", "--help"},
		{"browser", "eval", "--help"},
	}

	for _, g := range groups {
		t.Run(strings.Join(g, "_"), func(t *testing.T) {
			lastCode = 0
			switch g[0] {
			case "session":
				runSession(info, g[1:])
			case "browser":
				runBrowser(info, g[1:])
			case "hook":
				runHook(g[1:])
			case "checkpoint":
				runCheckpoint(g[1:])
			case "notebooklm":
				runNotebook(info, g[1:])
			}
			if lastCode != 0 {
				t.Errorf("help flag %v caused exit code %d, expected 0", g, lastCode)
			}
		})
	}
}

func TestCLI_ExitCodeContract_UsageErrors(t *testing.T) {
	// UX-6: Missing arguments or unknown subcommands must return ExitUsageError (2)
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var lastCode int
	exitFunc = func(code int) {
		lastCode = code
	}

	usageTests := []struct {
		name string
		run  func()
	}{
		{"session_empty", func() { runSession(info, []string{}) }},
		{"session_unknown", func() { runSession(info, []string{"bogus"}) }},
		{"browser_empty", func() { runBrowser(info, []string{}) }},
		{"browser_unknown", func() { runBrowser(info, []string{"bogus"}) }},
		{"browser_open_no_url", func() { runBrowser(info, []string{"open"}) }},
		{"browser_close_no_id", func() { runBrowser(info, []string{"close"}) }},
		{"notebook_empty", func() { runNotebook(info, []string{}) }},
		{"notebook_unknown", func() { runNotebook(info, []string{"bogus"}) }},
		{"notebook_push_missing_args", func() { runNotebook(info, []string{"push"}) }},
		{"hook_empty", func() { runHook([]string{}) }},
		{"hook_unknown", func() { runHook([]string{"bogus"}) }},
	}

	for _, tc := range usageTests {
		t.Run(tc.name, func(t *testing.T) {
			lastCode = 0
			tc.run()
			if lastCode != ExitUsageError {
				t.Errorf("expected exit code %d (ExitUsageError), got %d", ExitUsageError, lastCode)
			}
		})
	}
}

func TestCLI_ActionableHint_Port9222(t *testing.T) {
	// UX-8: runtime error on port 9222 must mention 'agyo browser start'
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var lastCode int
	exitFunc = func(code int) {
		lastCode = code
	}

	// Capture stderr
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	runtimeError(errors.New("dial tcp 127.0.0.1:9222: connect: connection refused"), "")
	w.Close()
	os.Stderr = origStderr

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if lastCode != ExitRuntimeError {
		t.Errorf("expected exit code %d (ExitRuntimeError), got %d", ExitRuntimeError, lastCode)
	}
	if !strings.Contains(output, "agyo browser start") {
		t.Errorf("expected hint mentioning 'agyo browser start', got: %s", output)
	}
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

func TestCLI_CuratedHelp_UX4(t *testing.T) {
	// UX-4: Group usage strings must contain descriptions and examples
	sessUsage := sessionUsageString()
	if !strings.Contains(sessUsage, "Examples:") || !strings.Contains(sessUsage, "agyo session status") {
		t.Errorf("session usage missing examples: %s", sessUsage)
	}

	browserUsage := browserUsageString()
	if !strings.Contains(browserUsage, "Examples:") || !strings.Contains(browserUsage, "agyo browser start") {
		t.Errorf("browser usage missing examples: %s", browserUsage)
	}

	chkUsage := checkpointUsageString()
	if !strings.Contains(chkUsage, "Examples:") || !strings.Contains(chkUsage, "-list") {
		t.Errorf("checkpoint usage missing examples or flags: %s", chkUsage)
	}

	rbUsage := rollbackUsageString()
	if !strings.Contains(rbUsage, "Examples:") || !strings.Contains(rbUsage, "latest") {
		t.Errorf("rollback usage missing examples or subcommands: %s", rbUsage)
	}
}

func TestCLI_JSONSupport_UX7(t *testing.T) {
	info := &platform.Info{
		OS:             "darwin",
		Arch:           "arm64",
		BrowserProfile: t.TempDir(),
	}

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	exitFunc = func(code int) {}

	// Helper to capture stdout
	captureStdout := func(f func()) string {
		origStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		f()

		w.Close()
		os.Stdout = origStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	// 1. browser status --json
	out := captureStdout(func() {
		runBrowser(info, []string{"status", "--json"})
	})
	var st map[string]interface{}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Errorf("browser status --json produced invalid JSON: %v, out: %s", err, out)
	}

	// 2. notebook status --json
	out = captureStdout(func() {
		runNotebook(info, []string{"status", "--json"})
	})
	var nbSt map[string]interface{}
	if err := json.Unmarshal([]byte(out), &nbSt); err != nil {
		t.Errorf("notebook status --json produced invalid JSON: %v, out: %s", err, out)
	}
}
