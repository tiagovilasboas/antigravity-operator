package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/checkpoint"
	"github.com/tiagovilasboas/antigravity-operator/internal/hook"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

func TestAntigravityHook_InstallDryRunAndReal(t *testing.T) {
	ws := t.TempDir()
	hooksFile := filepath.Join(ws, ".agents", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(hooksFile), 0755)

	// Existing third-party hook
	initialContent := `{
  "custom-router": {
    "enabled": true,
    "PreToolUse": []
  }
}`
	if err := os.WriteFile(hooksFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Dry run
	target, dryContent, err := hook.InstallAntigravityHook("workspace", ws, true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if target != hooksFile {
		t.Errorf("expected target %s, got %s", hooksFile, target)
	}
	if !strings.Contains(dryContent, hook.AgyoHookName) || !strings.Contains(dryContent, "custom-router") {
		t.Errorf("dry content missing keys: %s", dryContent)
	}

	// Ensure disk was NOT modified during dry-run
	diskData, _ := os.ReadFile(hooksFile)
	if strings.Contains(string(diskData), hook.AgyoHookName) {
		t.Errorf("dry-run wrote to disk")
	}

	// 2. Real install
	_, _, err = hook.InstallAntigravityHook("workspace", ws, false)
	if err != nil {
		t.Fatalf("real install failed: %v", err)
	}

	// Check backup was created
	backups, _ := filepath.Glob(hooksFile + ".bak-*")
	if len(backups) != 1 {
		t.Errorf("expected 1 backup file, found %d", len(backups))
	}

	// Verify status
	active, _, err := hook.AntigravityHookStatus("workspace", ws)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !active {
		t.Errorf("expected hook to be active")
	}

	// 3. Uninstall
	_, err = hook.UninstallAntigravityHook("workspace", ws)
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	active, _, _ = hook.AntigravityHookStatus("workspace", ws)
	if active {
		t.Errorf("expected hook to be inactive after uninstall")
	}

	// Ensure original custom-router is still preserved!
	finalDisk, _ := os.ReadFile(hooksFile)
	if !strings.Contains(string(finalDisk), "custom-router") {
		t.Errorf("custom-router was removed during uninstall")
	}
}

func TestAntigravityHook_PreInvocation_HK1(t *testing.T) {
	ws := t.TempDir()
	_, _ = session.Init(ws, true)

	// Put in-progress task in todo.md
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	todoContent := `# Lista de Tarefas da Sessão

## Em Progresso ⏳
- [ ] Implementar sensor de verificação
- [ ] Refatorar CLI

## Pendentes 📋
- [ ] Task 3
`
	_ = os.WriteFile(todoPath, []byte(todoContent), 0644)

	// Invocation 1
	in1 := &hook.PreInvocationInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-123",
			WorkspacePaths: []string{ws},
		},
		InvocationNum: 1,
	}

	out1, err := hook.HandlePreInvocation(context.Background(), in1)
	if err != nil {
		t.Fatalf("handle pre-invocation failed: %v", err)
	}

	if len(out1.InjectSteps) != 1 {
		t.Fatalf("expected 1 injected step, got %d", len(out1.InjectSteps))
	}

	msg := out1.InjectSteps[0].EphemeralMessage
	if !strings.Contains(msg, "Implementar sensor de verificação") {
		t.Errorf("injected message missing in-progress task: %s", msg)
	}

	// Invocation 2 (must NOT inject steps again)
	in2 := &hook.PreInvocationInput{
		CommonInput:   in1.CommonInput,
		InvocationNum: 2,
	}
	out2, err := hook.HandlePreInvocation(context.Background(), in2)
	if err != nil {
		t.Fatalf("handle pre-invocation 2 failed: %v", err)
	}
	if len(out2.InjectSteps) != 0 {
		t.Errorf("expected 0 injected steps on invocation 2, got %d", len(out2.InjectSteps))
	}
}

func TestAntigravityHook_PostToolUseAndStop_HK2_HK3(t *testing.T) {
	ws := t.TempDir()
	_, _ = session.Init(ws, true)
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	_ = os.WriteFile(todoPath, []byte("# Lista de Tarefas\n\n## Em Progresso ⏳\n\n## Pendentes 📋\n"), 0644)

	convID := "conv-test-stop"
	st := hook.LoadConversationState(convID, ws)
	st.DirtyCode = false
	st.StopBlocks = 0
	_ = st.Save()

	// 1. PostToolUse: edit a .go file -> marks DirtyCode = true
	postIn := &hook.PostToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "write_to_file",
			Args: map[string]interface{}{
				"TargetFile": "/path/to/main.go",
			},
		},
	}
	_, err := hook.HandlePostToolUse(context.Background(), postIn)
	if err != nil {
		t.Fatalf("PostToolUse failed: %v", err)
	}

	st = hook.LoadConversationState(convID, ws)
	if !st.DirtyCode {
		t.Errorf("expected dirtyCode to be true after editing .go file")
	}

	// 2. Stop: should block because dirtyCode is true
	stopIn := &hook.StopInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		TerminationReason: "model_stop",
	}
	stopOut, err := hook.HandleStop(context.Background(), stopIn)
	if err != nil {
		t.Fatalf("HandleStop failed: %v", err)
	}
	if stopOut.Decision != "continue" {
		t.Errorf("expected decision 'continue', got %q", stopOut.Decision)
	}
	if !strings.Contains(stopOut.Reason, "testes de verificação") {
		t.Errorf("expected reason to mention verification tests, got %q", stopOut.Reason)
	}

	// 3. PostToolUse: run test successfully -> marks DirtyCode = false
	testRunIn := &hook.PostToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": "go test -v ./...",
			},
		},
		Error: "", // success!
	}
	_, _ = hook.HandlePostToolUse(context.Background(), testRunIn)
	st = hook.LoadConversationState(convID, ws)
	if st.DirtyCode {
		t.Errorf("expected dirtyCode to be false after successful test command")
	}

	// 4. Now add an in-progress task to todo.md -> Stop should block again
	_ = os.WriteFile(todoPath, []byte("## Em Progresso ⏳\n- [ ] Pending work\n"), 0644)

	stopOut, _ = hook.HandleStop(context.Background(), stopIn)
	if stopOut.Decision != "continue" {
		t.Errorf("expected decision 'continue' due to in-progress task, got %q", stopOut.Decision)
	}

	// 5. Anti-loop test: repeated stop blocks must allow termination after MaxStopBlocks (2)
	// Currently StopBlocks is 2, next call should allow stopping
	stopOut, _ = hook.HandleStop(context.Background(), stopIn)
	if stopOut.Decision == "continue" {
		t.Errorf("anti-loop threshold should allow stop, but got decision 'continue'")
	}
}

func TestAntigravityHook_DispatchFailOpen(t *testing.T) {
	// DispatchHookEvent must be strictly fail-open: bad json, unknown event, etc. must output valid JSON and not crash
	var buf bytes.Buffer
	err := hook.DispatchHookEvent(context.Background(), "PreInvocation", strings.NewReader("invalid-json{"), &buf)
	if err != nil {
		t.Fatalf("expected nil error on fail-open dispatch, got: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON output on dispatch fail-open: %v", err)
	}

	// Unknown event
	buf.Reset()
	_ = hook.DispatchHookEvent(context.Background(), "unknown_event", strings.NewReader("{}"), &buf)
	if strings.TrimSpace(buf.String()) != "{}" {
		t.Errorf("expected {} on unknown event, got %s", buf.String())
	}
}

func TestAntigravityHook_PreInvocation_HK4_Conflict(t *testing.T) {
	ws := t.TempDir()
	_, _ = session.Init(ws, true)

	// Conversation 1 invokes
	in1 := &hook.PreInvocationInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-alpha",
			WorkspacePaths: []string{ws},
		},
		InvocationNum: 1,
	}
	out1, err := hook.HandlePreInvocation(context.Background(), in1)
	if err != nil {
		t.Fatalf("first invocation failed: %v", err)
	}
	if len(out1.InjectSteps) == 0 {
		t.Fatalf("expected injected steps")
	}
	// First conversation should not have conflict
	if strings.Contains(out1.InjectSteps[0].EphemeralMessage, "Conflito de sessão") {
		t.Errorf("first conversation should not trigger conflict")
	}

	// Conversation 2 invokes in the SAME workspace
	in2 := &hook.PreInvocationInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-beta",
			WorkspacePaths: []string{ws},
		},
		InvocationNum: 1,
	}
	out2, err := hook.HandlePreInvocation(context.Background(), in2)
	if err != nil {
		t.Fatalf("second invocation failed: %v", err)
	}
	if len(out2.InjectSteps) == 0 {
		t.Fatalf("expected injected steps")
	}
	// Second conversation MUST detect conflict with conv-alpha
	msg := out2.InjectSteps[0].EphemeralMessage
	if !strings.Contains(msg, "Conflito de sessão") || !strings.Contains(msg, "conv-alpha") {
		t.Errorf("expected conflict warning mentioning conv-alpha, got: %s", msg)
	}
}

func TestAntigravityHook_PreInvocation_HK5_CompactSuggestion(t *testing.T) {
	ws := t.TempDir()
	_, _ = session.Init(ws, true)

	// Add 6 done tasks to todo.md to trigger yellow zone compact suggestion
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	var sb strings.Builder
	sb.WriteString("# Tarefas\n\n## Concluídas ✅\n")
	for i := 1; i <= 6; i++ {
		sb.WriteString("- [x] Tarefa concluída\n")
	}
	_ = os.WriteFile(todoPath, []byte(sb.String()), 0644)

	in := &hook.PreInvocationInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-compaction-test",
			WorkspacePaths: []string{ws},
		},
		InvocationNum: 1,
	}
	out, err := hook.HandlePreInvocation(context.Background(), in)
	if err != nil {
		t.Fatalf("PreInvocation failed: %v", err)
	}
	if len(out.InjectSteps) == 0 {
		t.Fatalf("expected injected steps")
	}
	msg := out.InjectSteps[0].EphemeralMessage
	if !strings.Contains(msg, "agyo session compact") {
		t.Errorf("expected compact suggestion for 6 done tasks, got: %s", msg)
	}
}

func TestAntigravityHook_PreToolUse_HK6_DestructiveCheckpoint(t *testing.T) {
	ws := t.TempDir()
	// Init git repository
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Tester"},
		{"git", "config", "user.email", "tester@example.com"},
		{"git", "commit", "--allow-empty", "-m", "Initial commit"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v (%s)", c, err, string(out))
		}
	}

	// 1. Non-destructive command (ls -la) should NOT create checkpoint
	nonDestructiveIn := &hook.PreToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-chk-test",
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": "ls -la",
			},
		},
	}
	_, err := hook.HandlePreToolUse(context.Background(), nonDestructiveIn)
	if err != nil {
		t.Fatalf("HandlePreToolUse failed on non-destructive: %v", err)
	}

	chks, _ := checkpoint.List(ws)
	if len(chks) != 0 {
		t.Errorf("expected 0 checkpoints for non-destructive command, got %d", len(chks))
	}

	// 2. Destructive command (git reset --hard) MUST create an automated checkpoint
	destructiveIn := &hook.PreToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: "conv-chk-test",
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": "git reset --hard HEAD~1",
			},
		},
	}
	_, err = hook.HandlePreToolUse(context.Background(), destructiveIn)
	if err != nil {
		t.Fatalf("HandlePreToolUse failed on destructive: %v", err)
	}

	chks, err = checkpoint.List(ws)
	if err != nil {
		t.Fatalf("checkpoint.List failed: %v", err)
	}
	if len(chks) != 1 {
		t.Fatalf("expected 1 checkpoint created before destructive command, got %d", len(chks))
	}
	if chks[0].Name != "pre-destructive" {
		t.Errorf("expected checkpoint name 'pre-destructive', got %q", chks[0].Name)
	}
}

func TestAntigravityHook_PreToolUse_HK7_BrowserGuardrail(t *testing.T) {
	blockedCommands := []string{
		`open -a "Google Chrome" https://example.com`,
		`/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --profile-directory="Profile 3"`,
		`osascript -e 'tell application "Google Chrome" to tell front window to make new tab with properties {URL:"https://example.com"}'`,
		`osascript -e 'tell application "System Events" to tell process "Google Chrome" to click menu item "Profile"'`,
	}

	for _, cmdStr := range blockedCommands {
		in := &hook.PreToolUseInput{
			CommonInput: hook.CommonInput{ConversationID: "conv-hk7"},
			ToolCall: &hook.ToolCall{
				Name: "run_command",
				Args: map[string]interface{}{"CommandLine": cmdStr},
			},
		}
		out, err := hook.HandlePreToolUse(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", cmdStr, err)
		}
		if out.Decision != "block" {
			t.Errorf("expected command %q to be blocked by HK-7, got decision=%q", cmdStr, out.Decision)
		}
	}

	// Safe read-only or active-tab reuse commands must NOT be blocked
	allowedCommands := []string{
		`agyo browser profiles`,
		`agyo browser start --profile workspace`,
		`osascript -e 'tell application "Google Chrome" to get {title, URL} of active tab of front window'`,
		`osascript -e 'tell application "Google Chrome" to set URL of active tab of front window to "https://example.com"'`,
	}

	for _, cmdStr := range allowedCommands {
		in := &hook.PreToolUseInput{
			CommonInput: hook.CommonInput{ConversationID: "conv-hk7"},
			ToolCall: &hook.ToolCall{
				Name: "run_command",
				Args: map[string]interface{}{"CommandLine": cmdStr},
			},
		}
		out, err := hook.HandlePreToolUse(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", cmdStr, err)
		}
		if out.Decision == "block" {
			t.Errorf("expected command %q to be allowed, but was blocked: %s", cmdStr, out.Reason)
		}
	}
}
