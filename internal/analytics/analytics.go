package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/watcher"
)

// ToolCallRecord stores metadata about an individual tool invocation.
type ToolCallRecord struct {
	ID                int     `json:"id"`
	StepIndex         int     `json:"step_index"`
	Tool              string  `json:"tool"`
	Summary           string  `json:"summary,omitempty"`
	IsPromptTriggered bool    `json:"is_prompt_triggered"`
	IsAutonomous      bool    `json:"is_autonomous"`
	IsFailed          bool    `json:"is_failed"`
	Timestamp         string  `json:"timestamp,omitempty"`
	DurationSeconds   float64 `json:"duration_seconds,omitempty"`
}

// CommandStat captures execution frequency for a distinct shell command.
type CommandStat struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

// SessionAnalytics aggregates execution metrics and tool distribution.
type SessionAnalytics struct {
	ConversationID      string           `json:"conversation_id,omitempty"`
	Workspace           string           `json:"workspace,omitempty"`
	TotalSteps          int              `json:"total_steps"`
	ModelTurns          int              `json:"model_turns"`
	UserTurns           int              `json:"user_turns"`
	SystemMessages      int              `json:"system_messages"`
	TotalToolCalls      int              `json:"total_tool_calls"`
	AutonomousToolCalls int              `json:"autonomous_tool_calls"`
	PromptedToolCalls   int              `json:"prompted_tool_calls"`
	FailedToolCalls     int              `json:"failed_tool_calls"`
	ToolCounts          map[string]int   `json:"tool_counts"`
	TopCommands         []CommandStat    `json:"top_commands"`
	FilesRead           map[string]int   `json:"files_read,omitempty"`
	FilesModified       map[string]int   `json:"files_modified,omitempty"`
	ToolCalls           []ToolCallRecord `json:"tool_calls,omitempty"`
	StartTime           time.Time        `json:"start_time,omitempty"`
	LastActiveTime      time.Time        `json:"last_active_time,omitempty"`
	DurationSeconds     float64          `json:"duration_seconds"`
}

const (
	maxTrackedFiles     = 20
	maxDistinctCommands = 100
	maxToolCalls        = 500
	maxLineBytes        = 10 * 1024 * 1024 // 10 MB line limit
)

var (
	// PEM private key blocks
	rePEMBlock = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)

	// Standard Authorization and Bearer headers
	reAuthHeader = regexp.MustCompile(`(?i)\bAuthorization:\s*(Bearer\s+|Basic\s+)?[^\s'"]+`)
	reBearer     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-\._~\+\/]+=*`)

	// Token patterns: OpenAI (sk-, sk-ant-), Slack (xox*), Google (AIza), GitHub (ghp, github_pat), AWS (AKIA)
	reOpenAIKey    = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{20,}|sk-ant-[A-Za-z0-9_-]{20,})\b`)
	reSlackToken   = regexp.MustCompile(`\bxox[bpa]-[0-9A-Za-z_-]{10,}\b`)
	reGoogleKey    = regexp.MustCompile(`\bAIza[0-9A-Za-z-_]{30,45}\b`)
	reGitHubToken  = regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9_]{10,}|github_pat_[A-Za-z0-9_]{10,})\b`)
	reAWSAccessKey = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)

	// JWTs: eyJ...ey...
	reJWT = regexp.MustCompile(`\bey[A-Za-z0-9_-]{10,}\.ey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+\b`)

	// URL credentials: user:pass@ in authority only (cannot contain / or whitespace)
	reURLCreds = regexp.MustCompile(`(://[^:/\s'"]+):([^@/\s'"]+)@`)

	// curl -u user:pass (handles optional quotes around credentials)
	reCurlAuth = regexp.MustCompile(`(?i)(\bcurl\b[^\n|;&]*-u\s+["']?[^:\s'"]+):([^\s'"]+)(["']?)`)

	// CLI password flags restricted to database/auth contexts (mysql, mariadb, sshpass)
	reCliPass = regexp.MustCompile(`(?i)(\b(?:mysql|mysqldump|mariadb|sshpass)\b[^\n|;&]*-p\s*=?\s*)([^\s"'>;&]+)`)

	// Generic secret variable assignments: key[:=]value with optional quotes and prefixes (e.g. DB_PASSWORD=foo, "token": "xyz", :_authToken=npm_...)
	reGenericSecret = regexp.MustCompile(`(?i)(["']?[:_A-Za-z0-9_-]*(?:token|secret|password|passwd|api[_-]?key|auth[_-]?token)[:_A-Za-z0-9_-]*["']?\s*[:=]\s*)(["']?)([^\s"'>;&]+)(["']?)`)
)

// RedactSecrets replaces sensitive tokens, passwords, and API keys with [REDACTED].
func RedactSecrets(s string) string {
	if s == "" {
		return ""
	}
	s = rePEMBlock.ReplaceAllString(s, "[REDACTED PRIVATE KEY]")
	s = reAuthHeader.ReplaceAllString(s, "Authorization: [REDACTED]")
	s = reBearer.ReplaceAllString(s, "Bearer [REDACTED]")
	s = reOpenAIKey.ReplaceAllString(s, "[REDACTED]")
	s = reSlackToken.ReplaceAllString(s, "[REDACTED]")
	s = reGoogleKey.ReplaceAllString(s, "[REDACTED]")
	s = reGitHubToken.ReplaceAllString(s, "[REDACTED]")
	s = reAWSAccessKey.ReplaceAllString(s, "[REDACTED]")
	s = reJWT.ReplaceAllString(s, "[REDACTED]")
	s = reURLCreds.ReplaceAllString(s, "$1:[REDACTED]@")
	s = reCurlAuth.ReplaceAllString(s, "$1:[REDACTED]$3")
	s = reCliPass.ReplaceAllString(s, "${1}[REDACTED]")
	s = reGenericSecret.ReplaceAllString(s, "${1}${2}[REDACTED]${4}")
	return s
}

// Truncate cuts a string to maxRunes, adding an ellipsis if truncated.
// It is safely guarded against limits < 3.
func Truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes < 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// Sanitize applies secret redaction followed by length truncation.
func Sanitize(s string, maxRunes int) string {
	return Truncate(RedactSecrets(s), maxRunes)
}

func trackFile(m map[string]int, path string) {
	if _, exists := m[path]; exists {
		m[path]++
	} else if len(m) < maxTrackedFiles {
		m[path] = 1
	}
}

func recordCommand(freq map[string]int, cmd string) {
	clean := Sanitize(cmd, 120)
	if _, exists := freq[clean]; exists {
		freq[clean]++
	} else if len(freq) < maxDistinctCommands {
		freq[clean] = 1
	}
}

// AnalyzeTranscript reads a transcript.jsonl file and computes analytics metrics.
func AnalyzeTranscript(transcriptPath string) (*SessionAnalytics, error) {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript: %w", err)
	}
	defer file.Close()

	sa := &SessionAnalytics{
		ToolCounts:    make(map[string]int),
		FilesRead:     make(map[string]int),
		FilesModified: make(map[string]int),
	}

	commandFreq := make(map[string]int)
	reader := bufio.NewReaderSize(file, 64*1024)

	var nextPlannerIsPromptTriggered bool
	var pendingToolIndices []int
	toolCallCounter := 0

	for {
		var line []byte
		var oversized bool
		for {
			chunk, isPrefix, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("failed to read transcript: %w", err)
			}
			if !oversized {
				if len(line)+len(chunk) > maxLineBytes {
					oversized = true
					line = nil
				} else {
					line = append(line, chunk...)
				}
			}
			if !isPrefix {
				break
			}
		}
		if oversized {
			continue // skip line over 10MB gracefully
		}
		if len(line) == 0 {
			if _, err := reader.Peek(1); err != nil {
				break
			}
			continue
		}

		evt, err := watcher.ParseLine(line)
		if err != nil {
			continue
		}

		sa.TotalSteps++

		var stepTime time.Time
		if evt.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, evt.CreatedAt)
			if err == nil {
				stepTime = t
				if sa.StartTime.IsZero() || t.Before(sa.StartTime) {
					sa.StartTime = t
				}
				if t.After(sa.LastActiveTime) {
					sa.LastActiveTime = t
				}
			}
		}

		switch evt.Type {
		case "USER_INPUT":
			sa.UserTurns++
			pendingToolIndices = nil
			nextPlannerIsPromptTriggered = true

			if sa.Workspace == "" {
				if strings.Contains(evt.Content, "Active Workspaces:") {
					parts := strings.Split(evt.Content, "Active Workspaces:")
					if len(parts) > 1 {
						for _, l := range strings.Split(parts[1], "\n") {
							l = strings.TrimSpace(l)
							if strings.HasPrefix(l, "- ") {
								ws := strings.TrimSpace(strings.TrimPrefix(l, "- "))
								sa.Workspace = Sanitize(ws, 120)
								break
							}
						}
					}
				}
				if sa.Workspace == "" && strings.Contains(evt.Content, "Command Working Directory:") {
					parts := strings.Split(evt.Content, "Command Working Directory:")
					if len(parts) > 1 {
						lines := strings.Split(parts[1], "\n")
						if len(lines) > 0 {
							sa.Workspace = Sanitize(strings.TrimSpace(lines[0]), 120)
						}
					}
				}
			}

		case "PLANNER_RESPONSE":
			sa.ModelTurns++
			isPromptTurn := nextPlannerIsPromptTriggered
			nextPlannerIsPromptTriggered = false

			for _, call := range evt.ToolCalls {
				toolCallCounter++
				sa.TotalToolCalls++
				sa.ToolCounts[call.Name]++

				if isPromptTurn {
					sa.PromptedToolCalls++
				} else {
					sa.AutonomousToolCalls++
				}

				record := ToolCallRecord{
					ID:                toolCallCounter,
					StepIndex:         evt.StepIndex,
					Tool:              call.Name,
					IsPromptTriggered: isPromptTurn,
					IsAutonomous:      !isPromptTurn,
					Timestamp:         evt.CreatedAt,
				}

				switch call.Name {
				case "run_command":
					var args struct {
						CommandLine string `json:"CommandLine"`
						Cwd         string `json:"Cwd"`
					}
					if err := json.Unmarshal(call.Args, &args); err == nil && args.CommandLine != "" {
						recordCommand(commandFreq, args.CommandLine)
						if sa.Workspace == "" && args.Cwd != "" {
							sa.Workspace = Sanitize(strings.TrimSpace(args.Cwd), 120)
						}
					}

				case "view_file":
					var args struct {
						AbsolutePath string `json:"AbsolutePath"`
					}
					if err := json.Unmarshal(call.Args, &args); err == nil && args.AbsolutePath != "" {
						trackFile(sa.FilesRead, Sanitize(strings.TrimSpace(args.AbsolutePath), 120))
					}

				case "replace_file_content", "write_to_file":
					var args struct {
						TargetFile string `json:"TargetFile"`
					}
					if err := json.Unmarshal(call.Args, &args); err == nil && args.TargetFile != "" {
						trackFile(sa.FilesModified, Sanitize(strings.TrimSpace(args.TargetFile), 120))
					}

				case "schedule":
					var args struct {
						DurationSeconds int `json:"DurationSeconds"`
					}
					if err := json.Unmarshal(call.Args, &args); err == nil {
						record.Summary = fmt.Sprintf("Timer (%ds)", args.DurationSeconds)
					}
				}

				if len(sa.ToolCalls) < maxToolCalls {
					recIndex := len(sa.ToolCalls)
					sa.ToolCalls = append(sa.ToolCalls, record)
					pendingToolIndices = append(pendingToolIndices, recIndex)
				}
			}

		case "GENERIC":
			if len(pendingToolIndices) > 0 {
				idx := pendingToolIndices[0]
				pendingToolIndices = pendingToolIndices[1:]

				content := evt.Content
				isFailed := false

				if evt.Status == "ERROR" {
					isFailed = true
				} else if strings.Contains(content, "Encountered error in tool execution") {
					isFailed = true
				} else {
					var lastExitCodeStr string
					for _, l := range strings.Split(content, "\n") {
						trimmed := strings.TrimSpace(l)
						if strings.HasPrefix(trimmed, "The command exited with code ") {
							lastExitCodeStr = strings.TrimSuffix(strings.TrimPrefix(trimmed, "The command exited with code "), ".")
						}
					}
					if lastExitCodeStr != "" && lastExitCodeStr != "0" {
						isFailed = true
					}
				}

				if isFailed {
					sa.ToolCalls[idx].IsFailed = true
					sa.FailedToolCalls++
				}

				if !stepTime.IsZero() && sa.ToolCalls[idx].Timestamp != "" {
					tStart, err := time.Parse(time.RFC3339, sa.ToolCalls[idx].Timestamp)
					if err == nil && stepTime.After(tStart) {
						sa.ToolCalls[idx].DurationSeconds = stepTime.Sub(tStart).Seconds()
					}
				}
			}

		case "SYSTEM_MESSAGE":
			sa.SystemMessages++
		}
	}

	if !sa.StartTime.IsZero() && !sa.LastActiveTime.IsZero() {
		sa.DurationSeconds = sa.LastActiveTime.Sub(sa.StartTime).Seconds()
	}

	cmdKeys := make([]string, 0, len(commandFreq))
	for cmd := range commandFreq {
		cmdKeys = append(cmdKeys, cmd)
	}
	sort.Strings(cmdKeys)

	for _, cmd := range cmdKeys {
		sa.TopCommands = append(sa.TopCommands, CommandStat{Command: cmd, Count: commandFreq[cmd]})
	}
	sort.Slice(sa.TopCommands, func(i, j int) bool {
		if sa.TopCommands[i].Count != sa.TopCommands[j].Count {
			return sa.TopCommands[i].Count > sa.TopCommands[j].Count
		}
		return sa.TopCommands[i].Command < sa.TopCommands[j].Command
	})
	if len(sa.TopCommands) > 10 {
		sa.TopCommands = sa.TopCommands[:10]
	}

	return sa, nil
}
