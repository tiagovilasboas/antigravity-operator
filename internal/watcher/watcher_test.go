package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFindActiveTranscript(t *testing.T) {
	tempGemini := t.TempDir()
	brainDir := filepath.Join(tempGemini, "brain")

	// 1. Empty brain dir
	_, err := FindActiveTranscript(tempGemini)
	if err == nil {
		t.Error("expected error for empty brain dir")
	}

	// 2. Multiple conversations with different timestamps
	conv1 := filepath.Join(brainDir, "conv-1", ".system_generated", "logs")
	conv2 := filepath.Join(brainDir, "conv-2", ".system_generated", "logs")
	_ = os.MkdirAll(conv1, 0755)
	_ = os.MkdirAll(conv2, 0755)

	file1 := filepath.Join(conv1, "transcript.jsonl")
	file2 := filepath.Join(conv2, "transcript.jsonl")

	_ = os.WriteFile(file1, []byte(`{"step_index":1}`), 0644)
	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(file2, []byte(`{"step_index":2}`), 0644)

	latest, err := FindActiveTranscript(tempGemini)
	if err != nil {
		t.Fatalf("unexpected error finding transcript: %v", err)
	}

	if latest.ConversationID != "conv-2" {
		t.Errorf("expected conv-2 as latest, got %s", latest.ConversationID)
	}
	if latest.Path != file2 {
		t.Errorf("expected path %s, got %s", file2, latest.Path)
	}
}

func TestParseLine(t *testing.T) {
	// 1. Empty line
	if _, err := ParseLine([]byte("   \n")); err == nil {
		t.Error("expected error on empty line")
	}

	// 2. Invalid JSON
	if _, err := ParseLine([]byte("{invalid")); err == nil {
		t.Error("expected error on invalid json")
	}

	// 3. User input
	userJSON := `{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","content":"Hello Agent"}`
	evt, err := ParseLine([]byte(userJSON))
	if err != nil {
		t.Fatalf("unexpected error parsing user input: %v", err)
	}
	if evt.StepIndex != 1 || evt.Type != "USER_INPUT" || evt.Content != "Hello Agent" {
		t.Errorf("unexpected event content: %+v", evt)
	}

	// 4. Planner response with thinking and tool calls
	plannerJSON := `{
		"step_index": 2,
		"source": "MODEL",
		"type": "PLANNER_RESPONSE",
		"status": "DONE",
		"thinking": "Need to run test command",
		"tool_calls": [
			{"name": "run_command", "args": {"CommandLine": "go test ./..."}},
			{"name": "ask_question", "args": {"question": "Should I proceed?"}}
		]
	}`
	evtPlanner, err := ParseLine([]byte(plannerJSON))
	if err != nil {
		t.Fatalf("unexpected error parsing planner: %v", err)
	}
	if !evtPlanner.IsQuestion {
		t.Error("expected IsQuestion to be true when ask_question tool is called")
	}
	if !evtPlanner.IsDone {
		t.Error("expected IsDone to be true when status is DONE")
	}
	if len(evtPlanner.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls, got %d", len(evtPlanner.ToolCalls))
	}
}

func TestEventSummary(t *testing.T) {
	// 1. User input summary
	uEvt := &Event{StepIndex: 1, Type: "USER_INPUT", Content: "Run doctor"}
	if !strings.Contains(uEvt.Summary(), "Run doctor") {
		t.Errorf("expected summary to contain user content: %s", uEvt.Summary())
	}

	// 2. Planner with thinking and tools
	pEvt := &Event{
		StepIndex: 2,
		Type:      "PLANNER_RESPONSE",
		Thinking:  "Thinking deep...",
		ToolCalls: []ToolCall{
			{Name: "run_command", Args: []byte(`{"CommandLine":"git status"}`)},
			{Name: "view_file", Args: []byte(`{"AbsolutePath":"/path/to/main.go"}`)},
			{Name: "call_mcp_tool", Args: []byte(`{"ServerName":"mysql","ToolName":"query"}`)},
			{Name: "ask_question", Args: []byte(`{}`)},
		},
	}
	pSummary := pEvt.Summary()
	if !strings.Contains(pSummary, "Thinking deep...") || !strings.Contains(pSummary, "run_command(git status)") {
		t.Errorf("unexpected planner summary: %s", pSummary)
	}
	if !strings.Contains(pSummary, "main.go") || !strings.Contains(pSummary, "mysql/query") {
		t.Errorf("unexpected args formatting in summary: %s", pSummary)
	}

	// 3. Generic step
	gEvt := &Event{StepIndex: 3, Type: "GENERIC", Content: "Success output"}
	if !strings.Contains(gEvt.Summary(), "Success output") {
		t.Errorf("unexpected generic summary: %s", gEvt.Summary())
	}

	// 4. Default / unknown type
	dEvt := &Event{StepIndex: 4, Type: "CHECKPOINT", Status: "DONE"}
	if !strings.Contains(dEvt.Summary(), "CHECKPOINT") {
		t.Errorf("unexpected checkpoint summary: %s", dEvt.Summary())
	}

	// 5. extractArgsSummary test coverage
	if s := extractArgsSummary("write_to_file", []byte(`{"TargetFile":"/path/to/script.go"}`)); s != "script.go" {
		t.Errorf("expected script.go, got %s", s)
	}
	if s := extractArgsSummary("replace_file_content", []byte(`{"TargetFile":"/path/to/mod.go"}`)); s != "mod.go" {
		t.Errorf("expected mod.go, got %s", s)
	}
	if s := extractArgsSummary("unknown", []byte(`{"name":"foobar"}`)); s != "foobar" {
		t.Errorf("expected foobar, got %s", s)
	}
	if s := extractArgsSummary("empty", nil); s != "" {
		t.Errorf("expected empty string for nil args, got %s", s)
	}
	if s := extractArgsSummary("invalid", []byte(`invalid-json`)); s != "" {
		t.Errorf("expected empty string for invalid json, got %s", s)
	}
}

func TestNotify(t *testing.T) {
	var calls []string
	orig := NotifyExecutor
	defer func() { NotifyExecutor = orig }()
	NotifyExecutor = func(osName, title, message string) {
		calls = append(calls, osName+":"+title+":"+message)
	}

	Notify("mock-os", "Test Title", "Test Message")
	Notify("darwin", "Test Title", "Test Message")
	Notify("linux", "Test Title", "Test Message")

	if len(calls) != 3 {
		t.Errorf("esperava 3 chamadas de notificação, obteve %d", len(calls))
	}
}

func TestStream_InvalidFile(t *testing.T) {
	err := Stream(context.Background(), "/nonexistent/path/transcript.jsonl", WatchOptions{}, func(*Event) {})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestStream(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "transcript.jsonl")

	initialLines := `{"step_index":1,"type":"USER_INPUT","content":"Start"}
{"step_index":2,"type":"PLANNER_RESPONSE","thinking":"Planning"}
`
	_ = os.WriteFile(tempFile, []byte(initialLines), 0644)

	var received []*Event
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := WatchOptions{
		Follow:       true,
		PollInterval: 50 * time.Millisecond,
		InitialSteps: 2,
		OSName:       "mock",
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		// Append a new line dynamically
		f, _ := os.OpenFile(tempFile, os.O_APPEND|os.O_WRONLY, 0644)
		_, _ = f.WriteString("{\"step_index\":3,\"type\":\"GENERIC\",\"content\":\"Appended\"}\n")
		_ = f.Close()

		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	err := Stream(ctx, tempFile, opts, func(evt *Event) {
		received = append(received, evt)
	})

	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	if len(received) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(received))
	}
}

func TestSubagentAndMessageSummary(t *testing.T) {
	// 1. invoke_subagent event summary
	invokeEvt := &Event{
		StepIndex: 5,
		Type:      "PLANNER_RESPONSE",
		ToolCalls: []ToolCall{
			{
				Name: "invoke_subagent",
				Args: []byte(`{"Subagents":[{"Role":"Codebase Researcher","TypeName":"research","Prompt":"Search for authentication logic"}]}`),
			},
		},
	}
	summary := invokeEvt.Summary()
	if !strings.Contains(summary, "SUBAGENT SPAWNED") || !strings.Contains(summary, "Codebase Researcher") || !strings.Contains(summary, "research") {
		t.Errorf("unexpected invoke_subagent summary: %s", summary)
	}

	// 2. send_message event summary
	msgEvt := &Event{
		StepIndex: 6,
		Type:      "PLANNER_RESPONSE",
		ToolCalls: []ToolCall{
			{
				Name: "send_message",
				Args: []byte(`{"Recipient":"agent-abc","Message":"Task completed successfully"}`),
			},
		},
	}
	msgSummary := msgEvt.Summary()
	if !strings.Contains(msgSummary, "AGENT MESSAGE") || !strings.Contains(msgSummary, "agent-abc") || !strings.Contains(msgSummary, "Task completed successfully") {
		t.Errorf("unexpected send_message summary: %s", msgSummary)
	}

	// 3. extractArgsSummary for invoke_subagent and send_message
	if s := extractArgsSummary("invoke_subagent", []byte(`{"Subagents":[{"Role":"Debugger","TypeName":"debug"}]}`)); !strings.Contains(s, "Debugger") {
		t.Errorf("expected Debugger in args summary, got %s", s)
	}
	if s := extractArgsSummary("send_message", []byte(`{"Recipient":"worker-1"}`)); !strings.Contains(s, "worker-1") {
		t.Errorf("expected worker-1 in args summary, got %s", s)
	}
}

func TestSubagentTree(t *testing.T) {
	tree := NewSubagentTree()

	// Process invoke_subagent event
	evtSpawn := &Event{
		StepIndex: 1,
		Type:      "PLANNER_RESPONSE",
		ToolCalls: []ToolCall{
			{
				Name: "invoke_subagent",
				Args: []byte(`{"Subagents":[{"Role":"Codebase Researcher","TypeName":"research","Prompt":"Analyze DB schema"}]}`),
			},
		},
	}
	tree.ProcessEvent(evtSpawn)

	agents := tree.GetActiveSubagents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 active subagent, got %d", len(agents))
	}
	if agents[0].Role != "Codebase Researcher" || agents[0].TypeName != "research" {
		t.Errorf("unexpected subagent details: %+v", agents[0])
	}

	// Process send_message event
	evtMsg := &Event{
		StepIndex: 2,
		Type:      "PLANNER_RESPONSE",
		ToolCalls: []ToolCall{
			{
				Name: "send_message",
				Args: []byte(`{"Recipient":"Codebase Researcher","Message":"Found tables in db.go"}`),
			},
		},
	}
	tree.ProcessEvent(evtMsg)

	formatted := tree.FormatTree()
	if !strings.Contains(formatted, "Codebase Researcher") || !strings.Contains(formatted, "Found tables in db.go") {
		t.Errorf("unexpected formatted tree: %s", formatted)
	}
}

func TestSubagentTreeConcurrent(t *testing.T) {
	tree := NewSubagentTree()
	var wg sync.WaitGroup
	workers := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tree.AddSpawn(fmt.Sprintf("Worker %d", id), "worker", "Do work", "model", "workspace")
			tree.AddMessage("parent", fmt.Sprintf("Message from %d", id))
			_ = tree.GetActiveSubagents()
			_ = tree.FormatTree()
		}(i)
	}
	wg.Wait()
}

func TestFindLatestTranscript_AcrossAppDataDirs(t *testing.T) {
	home := t.TempDir()
	write := func(rel, conv string, mod time.Time) string {
		dir := filepath.Join(home, rel, "brain", conv, ".system_generated", "logs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "transcript.jsonl")
		if err := os.WriteFile(p, []byte(`{"step_index":1}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if _, err := FindLatestTranscript(home); err == nil {
		t.Fatal("expected an error when no app data dir has transcripts")
	}

	base := time.Now().Add(-time.Hour)
	write(".gemini/antigravity", "app", base)
	// ~/.gemini/brain is not an Antigravity location and must be ignored,
	// even when it holds the newest file.
	write(".gemini", "stray", base.Add(30*time.Minute))
	cli := write(".gemini/antigravity-cli", "cli", base.Add(10*time.Minute))

	got, err := FindLatestTranscript(home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != cli || got.ConversationID != "cli" {
		t.Fatalf("want newest CLI transcript %s, got %s (%s)", cli, got.Path, got.ConversationID)
	}

	ide := write(".gemini/antigravity-ide", "ide", base.Add(20*time.Minute))
	if got, _ = FindLatestTranscript(home); got.Path != ide {
		t.Fatalf("want newest IDE transcript %s, got %s", ide, got.Path)
	}

	dirs := AppDataDirs(home)
	if len(dirs) != 3 || dirs[0] != filepath.Join(home, ".gemini", "antigravity") {
		t.Fatalf("unexpected app data dirs: %v", dirs)
	}
}
