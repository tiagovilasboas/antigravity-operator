package analytics

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeTranscript(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	sampleContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"Please check the status"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"git status"}}]}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-01T10:00:06Z","content":"The command exited with code 0."}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:10Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"git status"}}]}
{"step_index":4,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-01T10:00:11Z","content":"The command exited with code 1."}
{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:15Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/tmp/test.go"}}]}
{"step_index":6,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-01T10:00:16Z","content":"File contents"}
{"step_index":7,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:20Z","tool_calls":[{"name":"schedule","args":{"DurationSeconds":45,"Prompt":"remind me later"}}]}
`
	if err := os.WriteFile(logPath, []byte(sampleContent), 0644); err != nil {
		t.Fatalf("failed to write test transcript: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if sa.TotalSteps != 8 {
		t.Errorf("expected 8 steps, got %d", sa.TotalSteps)
	}
	if sa.UserTurns != 1 {
		t.Errorf("expected 1 user turn, got %d", sa.UserTurns)
	}
	if sa.TotalToolCalls != 4 {
		t.Errorf("expected 4 tool calls, got %d", sa.TotalToolCalls)
	}
	if sa.FailedToolCalls != 1 {
		t.Errorf("expected 1 failed tool call, got %d", sa.FailedToolCalls)
	}

	// Verify Timer summary does NOT contain prompt text
	var timerCall *ToolCallRecord
	for i := range sa.ToolCalls {
		if sa.ToolCalls[i].Tool == "schedule" {
			timerCall = &sa.ToolCalls[i]
			break
		}
	}
	if timerCall == nil {
		t.Fatalf("expected schedule tool call")
	}
	if timerCall.Summary != "Timer (45s)" {
		t.Errorf("expected timer summary 'Timer (45s)', got %q", timerCall.Summary)
	}
}

func TestAnalyzeTranscript_Empty(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "empty.jsonl")
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("unexpected error on empty file: %v", err)
	}
	if sa.TotalSteps != 0 {
		t.Errorf("expected 0 steps, got %d", sa.TotalSteps)
	}
}

func TestAnalyzeTranscript_FIFOResetOnUserInput(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "fifo_reset.jsonl")

	content := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"First prompt"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"long-task"}}]}
{"step_index":2,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:10Z","content":"Cancel and do something else"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:15Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"make build"}}]}
{"step_index":4,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-01T10:00:20Z","content":"The command exited with code 1."}
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if len(sa.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(sa.ToolCalls))
	}
	if sa.ToolCalls[0].IsFailed {
		t.Errorf("first tool call should not be marked failed")
	}
	if !sa.ToolCalls[1].IsFailed {
		t.Errorf("second tool call should be marked failed")
	}
}

func TestAnalyzeTranscript_LastExitCodeTakesPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "last_exit_code.jsonl")

	content := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"Run script"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"./script.sh"}}]}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-01T10:00:10Z","content":"Substep 1 failed!\nThe command exited with code 1.\nSubstep 2 succeeded!\nThe command exited with code 0."}
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if sa.FailedToolCalls != 0 {
		t.Errorf("expected 0 failed tool calls because final exit was 0, got %d", sa.FailedToolCalls)
	}
	if len(sa.ToolCalls) > 0 && sa.ToolCalls[0].IsFailed {
		t.Errorf("expected tool call to not be marked failed")
	}
}

func TestAnalyzeTranscript_OversizedLineSkipped(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "oversized.jsonl")

	bigPadding := strings.Repeat("A", 11*1024*1024)
	oversizedLine := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"` + bigPadding + `"}`
	validLine := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"git status"}}]}`

	if err := os.WriteFile(logPath, []byte(oversizedLine+"\n"+validLine+"\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("expected oversized line to be skipped without error, got: %v", err)
	}
	if sa.TotalSteps != 1 {
		t.Errorf("expected 1 valid step after skipping oversized line, got %d", sa.TotalSteps)
	}
}

func TestAnalyzeTranscript_FilesCountIncrementedAfterCap(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "files_cap_increment.jsonl")

	var sb strings.Builder
	sb.WriteString(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"File test"}` + "\n")
	// 20 distinct files
	for i := 1; i <= 20; i++ {
		sb.WriteString(fmt.Sprintf(`{"step_index":%d,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/tmp/file%d.txt"}}]}`, i, i) + "\n")
	}
	// Add 5 more distinct files (should be gated)
	for i := 21; i <= 25; i++ {
		sb.WriteString(fmt.Sprintf(`{"step_index":%d,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/tmp/file%d.txt"}}]}`, i, i) + "\n")
	}
	// Re-visit /tmp/file1.txt 5 more times (already in map, must increment to 6)
	for i := 26; i <= 30; i++ {
		sb.WriteString(`{"step_index":26,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:05Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/tmp/file1.txt"}}]}` + "\n")
	}

	if err := os.WriteFile(logPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if len(sa.FilesRead) != 20 {
		t.Errorf("expected 20 unique files, got %d", len(sa.FilesRead))
	}
	if sa.FilesRead["/tmp/file1.txt"] != 6 {
		t.Errorf("expected /tmp/file1.txt to have count 6, got %d", sa.FilesRead["/tmp/file1.txt"])
	}
}

func TestAnalyzeTranscript_WorkspaceSanitized(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "workspace_sanitized.jsonl")

	content := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"Active Workspaces:\n- /home/user/project?token=secret12345"}` + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if strings.Contains(sa.Workspace, "secret12345") {
		t.Errorf("workspace contains unredacted secret: %q", sa.Workspace)
	}
	if !strings.Contains(sa.Workspace, "[REDACTED]") {
		t.Errorf("expected workspace to contain [REDACTED], got %q", sa.Workspace)
	}
}

func TestAnalyzeTranscript_TopCommandsOrderingAndCut(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "top_commands.jsonl")

	var sb strings.Builder
	sb.WriteString(`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-01T10:00:00Z","content":"Test commands"}` + "\n")

	for i := 0; i < 5; i++ {
		sb.WriteString(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:01Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"cmdA"}}]}` + "\n")
	}
	for i := 0; i < 3; i++ {
		sb.WriteString(`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:02Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"cmdC"}}]}` + "\n")
	}
	for i := 0; i < 3; i++ {
		sb.WriteString(`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:03Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"cmdB"}}]}` + "\n")
	}
	for i := 1; i <= 10; i++ {
		cmdName := fmt.Sprintf("cmdExtra%02d", i)
		sb.WriteString(fmt.Sprintf(`{"step_index":4,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-01T10:00:04Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"%s"}}]}`, cmdName) + "\n")
	}

	if err := os.WriteFile(logPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	sa, err := AnalyzeTranscript(logPath)
	if err != nil {
		t.Fatalf("AnalyzeTranscript failed: %v", err)
	}

	if len(sa.TopCommands) != 10 {
		t.Fatalf("expected TopCommands to be cut at 10, got %d", len(sa.TopCommands))
	}
	if sa.TopCommands[0].Command != "cmdA" || sa.TopCommands[0].Count != 5 {
		t.Errorf("expected cmdA with count 5 at index 0, got %+v", sa.TopCommands[0])
	}
	// Tie breaker: cmdB and cmdC both have count 3, cmdB < cmdC alphabetically
	if sa.TopCommands[1].Command != "cmdB" || sa.TopCommands[1].Count != 3 {
		t.Errorf("expected cmdB with count 3 at index 1, got %+v", sa.TopCommands[1])
	}
	if sa.TopCommands[2].Command != "cmdC" || sa.TopCommands[2].Count != 3 {
		t.Errorf("expected cmdC with count 3 at index 2, got %+v", sa.TopCommands[2])
	}
}

func TestSanitize(t *testing.T) {
	secretStr := "connect --DB_PASSWORD=supersecretpassword123 to database"
	got := Sanitize(secretStr, 35)
	expected := "connect --DB_PASSWORD=[REDACTED]..."
	if got != expected {
		t.Errorf("Sanitize got %q, want %q", got, expected)
	}

	// Guard limit < 3
	if got := Sanitize("hello", 2); got != "he" {
		t.Errorf("Sanitize got %q, want 'he'", got)
	}
	if got := Sanitize("hello", 1); got != "h" {
		t.Errorf("Sanitize got %q, want 'h'", got)
	}
}

func TestRedactSecrets_Table(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Authorization Bearer Header",
			input:    "curl -H 'Authorization: Bearer secret_token_123' https://api.example.com",
			expected: "curl -H 'Authorization: [REDACTED]' https://api.example.com",
		},
		{
			name:     "GitHub Token with Prefix",
			input:    "export GH_TOKEN=" + "ghp_" + "ABC1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "export GH_TOKEN=[REDACTED]",
		},
		{
			name:     "OpenAI API Key with Prefix",
			input:    "export OPENAI_API_KEY=" + "sk-" + "proj-1234567890abcdefghijklmnop",
			expected: "export OPENAI_API_KEY=[REDACTED]",
		},
		{
			name:     "Anthropic API Key standalone",
			input:    "sk-ant-" + "api03-1234567890abcdefghijklmnop",
			expected: "[REDACTED]",
		},
		{
			name:     "DB Password with Prefix",
			input:    "connect --DB_PASSWORD=supersecretpassword123",
			expected: "connect --DB_PASSWORD=[REDACTED]",
		},
		{
			name:     "JSON Quoted Password Value",
			input:    `{"DB_PASSWORD": "hunter2"}`,
			expected: `{"DB_PASSWORD": "[REDACTED]"}`,
		},
		{
			name:     "NPM Auth Token",
			input:    "//registry.npmjs.org/:_authToken=npm_1234567890abcdef",
			expected: "//registry.npmjs.org/:_authToken=[REDACTED]",
		},
		{
			name:     "AWS Secret Access Key",
			input:    "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expected: "AWS_SECRET_ACCESS_KEY=[REDACTED]",
		},
		{
			name:     "AWS Access Key ID standalone",
			input:    "aws configure set aws_access_key_id " + "AKIA" + "IOSFODNN7EXAMPLE",
			expected: "aws configure set aws_access_key_id [REDACTED]",
		},
		{
			name:     "URL Query Parameter access_token",
			input:    "https://api.example.com/data?access_token=secret12345",
			expected: "https://api.example.com/data?access_token=[REDACTED]",
		},
		{
			name:     "Slack Bot Token",
			input:    "xoxb" + "-1234567890-1234567890123-abcdefghijklmnop",
			expected: "[REDACTED]",
		},
		{
			name:     "Google API Key",
			input:    "AIza" + "SyD1234567890abcdefghijklmnopqrstuvw",
			expected: "[REDACTED]",
		},
		{
			name:     "JWT Token",
			input:    "token: " + "ey" + "JhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + "ey" + "JzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "token: [REDACTED]",
		},
		{
			name:     "Database URL with user:pass",
			input:    "postgres://admin:secretpass123@localhost:5432/mydb",
			expected: "postgres://admin:[REDACTED]@localhost:5432/mydb",
		},
		{
			name:     "curl -u user:pass unquoted",
			input:    "curl -u myuser:secretpass123 https://api.example.com",
			expected: "curl -u myuser:[REDACTED] https://api.example.com",
		},
		{
			name:     "curl -u user:pass quoted",
			input:    `curl -u "admin:hunter2" https://api.example.com`,
			expected: `curl -u "admin:[REDACTED]" https://api.example.com`,
		},
		{
			name:     "CLI password flag mysql -p",
			input:    "mysql -u root -psecretpassword123 dbname",
			expected: "mysql -u root -p[REDACTED] dbname",
		},
		{
			name:     "CLI password flag sshpass -p",
			input:    "sshpass -p hunter2 ssh user@host",
			expected: "sshpass -p [REDACTED] ssh user@host",
		},
		{
			name:     "PEM Private Key Block",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0abc...\n-----END RSA PRIVATE KEY-----",
			expected: "[REDACTED PRIVATE KEY]",
		},
		// Negative cases: MUST NOT be redacted
		{
			name:     "Negative: mkdir -p flag",
			input:    "mkdir -p /tmp/foo/bar",
			expected: "mkdir -p /tmp/foo/bar",
		},
		{
			name:     "Negative: git log -p flag",
			input:    "git log -p HEAD~3",
			expected: "git log -p HEAD~3",
		},
		{
			name:     "Negative: ripgrep password",
			input:    "rg password internal/",
			expected: "rg password internal/",
		},
		{
			name:     "Negative: grep token",
			input:    "grep -rn token src/",
			expected: "grep -rn token src/",
		},
		{
			name:     "Negative: echo monkey business",
			input:    "echo monkey business",
			expected: "echo monkey business",
		},
		{
			name:     "Negative: URL with @ in path",
			input:    "https://host:8080/a@b",
			expected: "https://host:8080/a@b",
		},
		{
			name:     "Negative: git checkout",
			input:    "git checkout -b feature-branch",
			expected: "git checkout -b feature-branch",
		},
		{
			name:     "Negative: keyboard layout",
			input:    "set keyboard layout to us",
			expected: "set keyboard layout to us",
		},
		{
			name:     "Negative: cat file",
			input:    "cat /etc/hosts",
			expected: "cat /etc/hosts",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := RedactSecrets(c.input)
			if out != c.expected {
				t.Errorf("RedactSecrets(%q)\ngot:  %q\nwant: %q", c.input, out, c.expected)
			}
		})
	}
}

func TestTruncate_Guards(t *testing.T) {
	if got := Truncate("hello", 0); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := Truncate("hello", -5); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := Truncate("hello", 2); got != "he" {
		t.Errorf("expected 'he', got %q", got)
	}
	if got := Truncate("hello", 1); got != "h" {
		t.Errorf("expected 'h', got %q", got)
	}
	if got := Truncate("hello world", 6); got != "hel..." {
		t.Errorf("expected 'hel...', got %q", got)
	}
}
