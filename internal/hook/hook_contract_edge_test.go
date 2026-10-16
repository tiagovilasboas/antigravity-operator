package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/hook"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

// TestQA_HookContract_FlatVsGrouped verifies that the exported hooks.json strictly obeys
// the Antigravity 2.0 contract: flat lists for PreInvocation and Stop, grouped lists for PreToolUse and PostToolUse.
func TestQA_HookContract_FlatVsGrouped(t *testing.T) {
	ws := t.TempDir()
	targetFile, serialized, err := hook.InstallAntigravityHook("workspace", ws, true)
	if err != nil {
		t.Fatalf("InstallAntigravityHook dry-run failed: %v", err)
	}

	if targetFile != filepath.Join(ws, ".agents", "hooks.json") {
		t.Errorf("unexpected target file: %s", targetFile)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(serialized), &parsed); err != nil {
		t.Fatalf("failed to parse generated hooks JSON: %v", err)
	}

	agyoHookRaw, exists := parsed[hook.AgyoHookName]
	if !exists {
		t.Fatalf("missing %q root entry in hooks config", hook.AgyoHookName)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(agyoHookRaw, &root); err != nil {
		t.Fatalf("failed to parse agyo hook object: %v", err)
	}

	// 1. PreInvocation must be a flat array of command objects
	preInvRaw, ok := root["PreInvocation"].([]interface{})
	if !ok || len(preInvRaw) == 0 {
		t.Fatalf("expected PreInvocation to be a non-empty array")
	}
	preInvItem, ok := preInvRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected PreInvocation[0] to be an object")
	}
	if _, hasHooks := preInvItem["hooks"]; hasHooks {
		t.Errorf("PreInvocation must be a flat list of commands, but found nested 'hooks' field")
	}
	if _, hasCmd := preInvItem["command"]; !hasCmd {
		t.Errorf("PreInvocation[0] must contain 'command' field directly")
	}

	// 2. Stop must be a flat array of command objects
	stopRaw, ok := root["Stop"].([]interface{})
	if !ok || len(stopRaw) == 0 {
		t.Fatalf("expected Stop to be a non-empty array")
	}
	stopItem, ok := stopRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Stop[0] to be an object")
	}
	if _, hasHooks := stopItem["hooks"]; hasHooks {
		t.Errorf("Stop must be a flat list of commands, but found nested 'hooks' field")
	}
	if _, hasCmd := stopItem["command"]; !hasCmd {
		t.Errorf("Stop[0] must contain 'command' field directly")
	}

	// 3. PreToolUse must be a grouped array with 'matcher' and nested 'hooks'
	preToolRaw, ok := root["PreToolUse"].([]interface{})
	if !ok || len(preToolRaw) == 0 {
		t.Fatalf("expected PreToolUse to be a non-empty array")
	}
	preToolItem, ok := preToolRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected PreToolUse[0] to be an object")
	}
	if _, hasMatcher := preToolItem["matcher"]; !hasMatcher {
		t.Errorf("PreToolUse must contain 'matcher' field")
	}
	nestedHooks, ok := preToolItem["hooks"].([]interface{})
	if !ok || len(nestedHooks) == 0 {
		t.Errorf("PreToolUse must contain nested 'hooks' array")
	}

	// 4. PostToolUse must be a grouped array with 'matcher' and nested 'hooks'
	postToolRaw, ok := root["PostToolUse"].([]interface{})
	if !ok || len(postToolRaw) == 0 {
		t.Fatalf("expected PostToolUse to be a non-empty array")
	}
	postToolItem, ok := postToolRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected PostToolUse[0] to be an object")
	}
	if _, hasMatcher := postToolItem["matcher"]; !hasMatcher {
		t.Errorf("PostToolUse must contain 'matcher' field")
	}
	nestedPostHooks, ok := postToolItem["hooks"].([]interface{})
	if !ok || len(nestedPostHooks) == 0 {
		t.Errorf("PostToolUse must contain nested 'hooks' array")
	}
}

// TestQA_PreToolUse_DecisionContract ensures PreToolUse emits strictly 'allow' or 'deny'.
func TestQA_PreToolUse_DecisionContract(t *testing.T) {
	ctx := context.Background()

	// 1. Rogue command should return decision 'deny'
	denyIn := &hook.PreToolUseInput{
		CommonInput: hook.CommonInput{ConversationID: "conv-qa-deny"},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": `open -a "Google Chrome" https://example.com`,
			},
		},
	}
	denyOut, err := hook.HandlePreToolUse(ctx, denyIn)
	if err != nil {
		t.Fatalf("HandlePreToolUse returned unexpected error: %v", err)
	}
	if denyOut.Decision != "deny" {
		t.Errorf("expected decision 'deny' for rogue browser command, got %q", denyOut.Decision)
	}
	if denyOut.Reason == "" {
		t.Errorf("expected descriptive reason when decision is 'deny'")
	}

	// 2. Allowed command should return decision 'allow'
	allowIn := &hook.PreToolUseInput{
		CommonInput: hook.CommonInput{ConversationID: "conv-qa-allow"},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": "git status",
			},
		},
	}
	allowOut, err := hook.HandlePreToolUse(ctx, allowIn)
	if err != nil {
		t.Fatalf("HandlePreToolUse returned unexpected error: %v", err)
	}
	if allowOut.Decision != "allow" {
		t.Errorf("expected decision 'allow' for benign command, got %q", allowOut.Decision)
	}

	// 3. Serialized JSON format must include decision field
	var buf bytes.Buffer
	inputData := `{"toolCall":{"name":"run_command","args":{"CommandLine":"git status"}}}`
	if err := hook.DispatchHookEvent(ctx, "PreToolUse", strings.NewReader(inputData), &buf); err != nil {
		t.Fatalf("DispatchHookEvent failed: %v", err)
	}
	var outMap map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &outMap); err != nil {
		t.Fatalf("failed to unmarshal PreToolUse stdout: %v", err)
	}
	if outMap["decision"] != "allow" {
		t.Errorf("expected decision='allow' in serialized JSON, got: %v", outMap["decision"])
	}
}

// TestQA_HookFailOpen_ExtremeInputs tests robustness under extreme and malformed inputs.
func TestQA_HookFailOpen_ExtremeInputs(t *testing.T) {
	ctx := context.Background()

	extremeInputs := []string{
		"",
		" ",
		"{",
		"[]",
		`{"conversationId": null, "workspacePaths": [null]}`,
		`{"toolCall": {"name": "", "args": null}}`,
		`{"invocationNum": -999}`,
		`{"terminationReason": ""}`,
	}

	events := []string{"PreInvocation", "PreToolUse", "PostToolUse", "Stop", "UnknownEvent"}

	for _, evt := range events {
		for _, input := range extremeInputs {
			var buf bytes.Buffer
			err := hook.DispatchHookEvent(ctx, evt, strings.NewReader(input), &buf)
			if err != nil {
				t.Errorf("DispatchHookEvent(%q, %q) returned error: %v", evt, input, err)
			}
			var parsed interface{}
			if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
				t.Errorf("DispatchHookEvent(%q, %q) produced invalid JSON: %s (err: %v)", evt, input, buf.String(), err)
			}
		}
	}
}

// TestQA_Antigravity2_DirtyCodeFlow simulates the full life cycle of an edit-test-stop loop.
func TestQA_Antigravity2_DirtyCodeFlow(t *testing.T) {
	ws := t.TempDir()
	_, _ = session.Init(ws, true)
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	_ = os.WriteFile(todoPath, []byte("# Lista de Tarefas\n\n## Em Progresso ⏳\n\n## Pendentes 📋\n"), 0644)

	convID := "conv-qa-cycle-01"

	st := hook.LoadConversationState(convID, ws)
	st.DirtyCode = false
	st.StopBlocks = 0
	_ = st.Save()

	ctx := context.Background()

	// Step 1: PreToolUse mutates a Go source file
	preWriteIn := &hook.PreToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "replace_file_content",
			Args: map[string]interface{}{
				"TargetFile": filepath.Join(ws, "main.go"),
			},
		},
	}
	preWriteOut, err := hook.HandlePreToolUse(ctx, preWriteIn)
	if err != nil || preWriteOut.Decision != "allow" {
		t.Fatalf("pre-write failed: %v, decision: %s", err, preWriteOut.Decision)
	}

	// Verify DirtyCode is flagged immediately
	st = hook.LoadConversationState(convID, ws)
	if !st.DirtyCode {
		t.Fatalf("expected DirtyCode=true immediately after PreToolUse on main.go")
	}

	// Step 2: PostToolUse arrives from Antigravity 2.0 without toolCall
	postAg2In := &hook.PostToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		StepIdx:  1,
		ToolCall: nil,
	}
	_, err = hook.HandlePostToolUse(ctx, postAg2In)
	if err != nil {
		t.Fatalf("post-write failed: %v", err)
	}

	// Verify DirtyCode was preserved
	st = hook.LoadConversationState(convID, ws)
	if !st.DirtyCode {
		t.Fatalf("DirtyCode was improperly cleared by Antigravity 2.0 nil-toolCall payload")
	}

	// Step 3: Model attempts to stop prematurely -> MUST BE BLOCKED
	stopIn := &hook.StopInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		TerminationReason: "model_stop",
	}
	stopOut, err := hook.HandleStop(ctx, stopIn)
	if err != nil {
		t.Fatalf("HandleStop failed: %v", err)
	}
	if stopOut.Decision != "continue" {
		t.Fatalf("expected Stop to be blocked with decision 'continue', got %q", stopOut.Decision)
	}

	// Step 4: Verification test is executed
	testRunIn := &hook.PostToolUseInput{
		CommonInput: hook.CommonInput{
			ConversationID: convID,
			WorkspacePaths: []string{ws},
		},
		ToolCall: &hook.ToolCall{
			Name: "run_command",
			Args: map[string]interface{}{
				"CommandLine": "go test ./...",
			},
		},
		Error: "",
	}
	_, _ = hook.HandlePostToolUse(ctx, testRunIn)

	// Step 5: Stop is re-evaluated -> MUST NOW BE PERMITTED
	stopOutAfterTest, err := hook.HandleStop(ctx, stopIn)
	if err != nil {
		t.Fatalf("HandleStop after test failed: %v", err)
	}
	if stopOutAfterTest.Decision == "continue" {
		t.Fatalf("Stop should have been allowed after successful test, but got decision 'continue'")
	}
}
