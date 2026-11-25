package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/checkpoint"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

const (
	MaxStopBlocks    = 2
	MaxPromptSummary = 1500 // 1.5 KB budget for PreInvocation ephemeral message
)

// CommonInput represents standard metadata sent by Google Antigravity lifecycle hooks.
type CommonInput struct {
	ConversationID        string   `json:"conversationId"`
	WorkspacePaths        []string `json:"workspacePaths"`
	TranscriptPath        string   `json:"transcriptPath"`
	ArtifactDirectoryPath string   `json:"artifactDirectoryPath"`
	ModelName             string   `json:"modelName"`
}

// PreInvocationInput is passed to the PreInvocation lifecycle event.
type PreInvocationInput struct {
	CommonInput
	InvocationNum   int `json:"invocationNum"`
	InitialNumSteps int `json:"initialNumSteps"`
}

// StepInjection defines a step to inject into the model context.
type StepInjection struct {
	EphemeralMessage string `json:"ephemeralMessage,omitempty"`
	UserMessage      string `json:"userMessage,omitempty"`
}

// PreInvocationOutput is returned by the PreInvocation handler.
type PreInvocationOutput struct {
	InjectSteps []StepInjection `json:"injectSteps,omitempty"`
}

// ToolCall represents tool call details in PostToolUse/PreToolUse.
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// PreToolUseInput is passed to the PreToolUse lifecycle event.
type PreToolUseInput struct {
	CommonInput
	StepIdx  int       `json:"stepIdx"`
	ToolCall *ToolCall `json:"toolCall,omitempty"`
}

// PreToolUseOutput is returned by the PreToolUse handler.
type PreToolUseOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// PostToolUseInput is passed to the PostToolUse lifecycle event.
type PostToolUseInput struct {
	CommonInput
	StepIdx  int       `json:"stepIdx"`
	Error    string    `json:"error,omitempty"`
	ToolCall *ToolCall `json:"toolCall,omitempty"`
}

// PostToolUseOutput is returned by the PostToolUse handler.
type PostToolUseOutput struct{}

// StopInput is passed to the Stop lifecycle event.
type StopInput struct {
	CommonInput
	ExecutionNum      int    `json:"executionNum"`
	TerminationReason string `json:"terminationReason"`
	Error             string `json:"error,omitempty"`
	FullyIdle         bool   `json:"fullyIdle"`
}

// StopOutput is returned by the Stop handler.
type StopOutput struct {
	Decision string `json:"decision,omitempty"` // "continue" to block stopping
	Reason   string `json:"reason,omitempty"`
}

var (
	codeExtRegex            = regexp.MustCompile(`\.(go|ts|tsx|js|jsx|py|rs|rb|php|java|c|cpp|h|hpp|cs)$`)
	testBuildCommandRegex   = regexp.MustCompile(`(?i)\b(go test|make test|npm test|pytest|cargo test|yarn test|pnpm test|vitest|go build|tsc|make build)\b`)
	destructiveCommandRegex = regexp.MustCompile(`(?i)\b(rm\s+-[a-zA-Z]*r[a-zA-Z]*f|git\s+reset\s+--hard|git\s+clean\s+-[a-zA-Z]*f|drop\s+database|drop\s+table|truncate\s+table)\b`)
	rogueBrowserSpawnRegex  = regexp.MustCompile(`(?i)(open\s+.*(-a\s+["']?(Google Chrome|Chrome)["']?|-b\s+com\.google\.Chrome)|(google-chrome|chromium(-browser)?|Google\s+Chrome\.app\/Contents\/MacOS\/Google\s+Chrome)|--profile-directory=|osascript[\s\S]*(Google Chrome|Chrome)[\s\S]*(make\s+new\s+(tab|window)|click\s+menu\s+item|open\s+location))`)
)

// HandlePreInvocation implements HK-1, HK-4, and HK-5: injecting session objective, checking workspace conflicts, and suggesting compaction.
func HandlePreInvocation(ctx context.Context, in *PreInvocationInput) (*PreInvocationOutput, error) {
	out := &PreInvocationOutput{InjectSteps: []StepInjection{}}
	// Only inject on invocation 1 or when explicitly starting
	if in.InvocationNum > 1 {
		return out, nil
	}
	ws := TargetWorkspace(in.WorkspacePaths)
	summary, err := session.GetSummary(ws)
	if err != nil {
		// Session memory does not exist in target workspace - silently skip
		return out, nil
	}
	var sb strings.Builder
	sb.WriteString("🎯 [agyo session memory] Contexto Operacional Ativo:\n")
	if summary.Objective != "" {
		fmt.Fprintf(&sb, "• Objetivo: %s\n", summary.Objective)
	}
	if summary.Status != "" {
		fmt.Fprintf(&sb, "• Status/Fase: %s\n", summary.Status)
	}
	// HK-4: Check concurrent conversation conflict in active workspace
	if in.ConversationID != "" {
		if prevConv, conflict := RegisterWorkspaceConversation(ws, in.ConversationID); conflict {
			fmt.Fprintf(&sb, "⚠️ [agyo] Conflito de sessão detectado: outra conversa (%s) esteve ativa recentemente neste workspace. A memória operacional (.agents/session/) é compartilhada.\n", prevConv)
		}
	}
	// Read active todo.md to collect In Progress tasks
	if inProgressTasks := extractInProgressTasks(filepath.Join(ws, ".agents", "session", "todo.md")); len(inProgressTasks) > 0 {
		sb.WriteString("• Tarefas em Progresso:\n")
		for _, t := range inProgressTasks {
			fmt.Fprintf(&sb, "  - %s\n", t)
		}
	}
	// HK-5: Suggest compaction if session footprint is in yellow or red zone, or done tasks >= 5
	if summary.HealthStatus == "Moderate 🟡" || summary.HealthStatus == "Bloated 🔴" || summary.DoneTasks >= 5 {
		fmt.Fprintf(&sb, "💡 [agyo] Pegada de sessão em zona moderada/crítica (%d tarefas concluídas, ~%d tokens, %s). Execute 'agyo session compact' para evitar context bloat.\n", summary.DoneTasks, summary.EstimatedTokens, summary.HealthStatus)
	}
	// HK-8: Observe & inject Multi-Profile Chrome map when multiple authenticated profiles exist
	if pInfo, pErr := platform.Detect(); pErr == nil {
		profs, _ := profile.DiscoverProfiles(pInfo.BrowserProfile, "isolated")
		if len(profs) == 0 {
			profs, _ = profile.DiscoverProfiles(pInfo.SystemChromeDir, "system")
		}
		var authLabels []string
		for _, p := range profs {
			if p.Kind != profile.ProfileKindGhost {
				if p.HostedDomain != "" {
					authLabels = append(authLabels, fmt.Sprintf("%s (%s: %s)", p.Directory, p.Kind, p.HostedDomain))
				} else {
					authLabels = append(authLabels, fmt.Sprintf("%s (%s)", p.Directory, p.Kind))
				}
			}
		}
		if len(authLabels) > 1 {
			fmt.Fprintf(&sb, "🧭 [agyo] Multi-Profile Chrome ativo: %s. Use 'agyo browser open <url>' ou 'agyo browser resolve <url>' para roteamento automático sem abrir janelas fantasmas.\n", strings.Join(authLabels, ", "))
		}
	}
	msg := sb.String()
	if len(msg) > MaxPromptSummary {
		msg = msg[:MaxPromptSummary-120] + "\n...(resumo truncado)\n⚠️ Memória operacional extensa (>1.5 KB). Execute 'agyo session compact' para poupar tokens de contexto.\n"
	}
	out.InjectSteps = append(out.InjectSteps, StepInjection{EphemeralMessage: msg})
	return out, nil
}

func isWriteFileTool(toolName string) bool {
	switch toolName {
	case "write_to_file", "replace_file_content", "write_file", "edit_file":
		return true
	}
	return strings.HasSuffix(toolName, "_write_file") || strings.HasSuffix(toolName, "_edit_file")
}

// HandlePreToolUse implements HK-6 (automated checkpoint before destructive commands),
// HK-7 (Single-Window & Multi-Profile Browser Guardrail sensor), and HK-3 (proactive dirty code tracking).
func HandlePreToolUse(ctx context.Context, in *PreToolUseInput) (*PreToolUseOutput, error) {
	if in.ToolCall == nil {
		return &PreToolUseOutput{Decision: "allow"}, nil
	}
	toolName := strings.ToLower(in.ToolCall.Name)
	// HK-3: Proactive dirty code tracking before writing code files
	if isWriteFileTool(toolName) {
		if targetFile := extractTargetFilePath(in.ToolCall.Args); targetFile != "" && codeExtRegex.MatchString(targetFile) {
			st := LoadConversationState(in.ConversationID, TargetWorkspace(in.WorkspacePaths))
			st.DirtyCode = true
			_ = st.Save()
		}
	}
	if toolName == "run_command" || toolName == "execute_bash" || toolName == "shell" {
		cmdStr := extractCommandLine(in.ToolCall.Args)
		// HK-7: Block rogue Chrome window/tab/profile spawning commands
		if rogueBrowserSpawnRegex.MatchString(cmdStr) {
			return &PreToolUseOutput{Decision: "deny", Reason: "Bloqueado pelo guardrail de Instância Única e Multi-Profile do agyo: é proibido abrir janelas paralelas ('open -a Google Chrome'), criar abas avulsas ('make new tab') ou alternar perfis via menu AppleScript. Use 'agyo browser profiles' e 'agyo browser resolve <url>' para descobrir o perfil autenticado correto, 'agyo browser start --profile <seletor>' para o Chrome isolado (CDP 9222), ou reutilize estritamente a aba ativa existente."}, nil
		}
		// HK-6: Automated checkpoint before destructive commands
		if destructiveCommandRegex.MatchString(cmdStr) {
			ws := TargetWorkspace(in.WorkspacePaths)
			if _, err := checkpoint.Create(ws, "pre-destructive", fmt.Sprintf("Automatic safety checkpoint before: %s", cmdStr)); err != nil {
				LogHookError("pretooluse-checkpoint", err)
			}
		}
	}
	return &PreToolUseOutput{Decision: "allow"}, nil
}

// HandlePostToolUse implements HK-3: tracking code edits vs verification runs per conversation.
func HandlePostToolUse(ctx context.Context, in *PostToolUseInput) (*PostToolUseOutput, error) {
	st := LoadConversationState(in.ConversationID, TargetWorkspace(in.WorkspacePaths))
	if in.ToolCall != nil {
		toolName := strings.ToLower(in.ToolCall.Name)
		// 1. Check if tool wrote or modified code
		if isWriteFileTool(toolName) {
			if targetFile := extractTargetFilePath(in.ToolCall.Args); targetFile != "" && codeExtRegex.MatchString(targetFile) {
				st.DirtyCode = true
			}
		}
		// 2. Check if tool executed a test or build command successfully
		if toolName == "run_command" || toolName == "execute_bash" || toolName == "shell" {
			cmdStr := extractCommandLine(in.ToolCall.Args)
			if testBuildCommandRegex.MatchString(cmdStr) && in.Error == "" {
				st.DirtyCode = false
			}
		}
	}
	_ = st.Save()
	return &PostToolUseOutput{}, nil
}

// HandleStop implements HK-2: blocking agent stop when tasks are in progress or code is unverified.
func HandleStop(ctx context.Context, in *StopInput) (*StopOutput, error) {
	// Only gate when the model explicitly requested stop
	if in.TerminationReason != "model_stop" {
		return &StopOutput{}, nil
	}
	ws := TargetWorkspace(in.WorkspacePaths)
	st := LoadConversationState(in.ConversationID, ws)
	// Anti-loop protection: allow stop if threshold reached
	if st.StopBlocks >= MaxStopBlocks {
		return &StopOutput{}, nil
	}
	// Check 1: In-progress tasks in todo.md
	if inProgress := extractInProgressTasks(filepath.Join(ws, ".agents", "session", "todo.md")); len(inProgress) > 0 {
		st.StopBlocks++
		_ = st.Save()
		return &StopOutput{Decision: "continue", Reason: fmt.Sprintf("O arquivo .agents/session/todo.md possui %d tarefa(s) marcadas 'Em Progresso' ([ ]). Conclua a tarefa e marque [x], ou desmarque antes de finalizar: %s", len(inProgress), strings.Join(inProgress, "; "))}, nil
	}
	// Check 2: Unverified code edits
	if st.DirtyCode {
		st.StopBlocks++
		_ = st.Save()
		return &StopOutput{Decision: "continue", Reason: "Arquivos de código foram modificados nesta sessão sem execução de testes de verificação ou compilação de sucesso (ex: go test, npm test, pytest, make build). Execute as verificações antes de encerrar."}, nil
	}
	return &StopOutput{}, nil
}

func extractTargetFilePath(args map[string]interface{}) string {
	return extractStringArg(args, "TargetFile", "targetFile", "path", "file_path", "filePath")
}

func extractCommandLine(args map[string]interface{}) string {
	return extractStringArg(args, "CommandLine", "commandLine", "command", "cmd")
}

func extractStringArg(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := args[key].(string); ok {
			return s
		}
	}
	return ""
}

func extractInProgressTasks(todoPath string) []string {
	data, err := os.ReadFile(todoPath)
	if err != nil {
		return nil
	}
	var tasks []string
	inProgress := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Em Progresso") {
			inProgress = true
		} else if strings.HasPrefix(trimmed, "## ") {
			inProgress = false
		} else if inProgress && strings.HasPrefix(trimmed, "- [ ]") {
			if task := strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ]")); task != "" {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

func dispatchTyped[In any, Out any](ctx context.Context, event string, raw []byte, w io.Writer, fn func(context.Context, *In) (*Out, error), fallback *Out) {
	var in In
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			LogHookError(event, fmt.Errorf("unmarshal error: %w", err))
		}
	}
	out, err := fn(ctx, &in)
	if err != nil {
		LogHookError(event, err)
		out = fallback
	}
	res, _ := json.Marshal(out)
	_, _ = w.Write(append(res, '\n'))
}

// DispatchHookEvent reads stdin JSON, dispatches to the requested event handler, and writes stdout JSON.
// In accordance with HK-0, it is strictly fail-open (never exits non-zero, logs errors).
func DispatchHookEvent(ctx context.Context, event string, r io.Reader, w io.Writer) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		LogHookError(event, fmt.Errorf("read stdin failed: %w", err))
		_, _ = io.WriteString(w, "{}\n")
		return nil
	}
	switch strings.ToLower(event) {
	case "preinvocation":
		dispatchTyped(ctx, event, raw, w, HandlePreInvocation, &PreInvocationOutput{})
	case "pretooluse":
		dispatchTyped(ctx, event, raw, w, HandlePreToolUse, &PreToolUseOutput{Decision: "allow"})
	case "posttooluse":
		dispatchTyped(ctx, event, raw, w, HandlePostToolUse, &PostToolUseOutput{})
	case "stop":
		dispatchTyped(ctx, event, raw, w, HandleStop, &StopOutput{})
	default:
		LogHookError(event, fmt.Errorf("unknown hook event: %s", event))
		_, _ = io.WriteString(w, "{}\n")
	}
	return nil
}
