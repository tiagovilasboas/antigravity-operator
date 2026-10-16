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
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/checkpoint"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
)

const (
	AgyoHookName     = "agyo-session-agent"
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

// ConversationState persists hook state across invocations for a single conversation.
type ConversationState struct {
	ConversationID string    `json:"conversationId"`
	WorkspacePath  string    `json:"workspacePath"`
	DirtyCode      bool      `json:"dirtyCode"`  // true if code modified without successful test/build
	StopBlocks     int       `json:"stopBlocks"` // anti-loop counter (max MaxStopBlocks)
	LastUpdated    time.Time `json:"lastUpdated"`
}

// Helper to resolve the hooks storage directory (~/.agyo/hooks/).
func hooksStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".agyo", "hooks")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// Helper to log hook failures fail-open.
func LogHookError(event string, err error) {
	if err == nil {
		return
	}
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".agyo", "hook-errors.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0700)
	f, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if openErr != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("[%s] [%s] %v\n", time.Now().Format(time.RFC3339), event, err)
	_, _ = f.WriteString(line)
}

// LoadConversationState loads or initializes state for a conversation.
func LoadConversationState(convID string, workspace string) *ConversationState {
	if convID == "" {
		convID = "default"
	}
	path := filepath.Join(hooksStateDir(), convID+".json")
	data, err := os.ReadFile(path)
	if err == nil {
		var st ConversationState
		if json.Unmarshal(data, &st) == nil {
			if workspace != "" && st.WorkspacePath == "" {
				st.WorkspacePath = workspace
			}
			return &st
		}
	}
	return &ConversationState{
		ConversationID: convID,
		WorkspacePath:  workspace,
		LastUpdated:    time.Now(),
	}
}

// Save persists the conversation state to disk.
func (s *ConversationState) Save() error {
	if s.ConversationID == "" {
		s.ConversationID = "default"
	}
	s.LastUpdated = time.Now()
	path := filepath.Join(hooksStateDir(), s.ConversationID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// TargetWorkspace resolves the active workspace directory from inputs.
func TargetWorkspace(paths []string) string {
	if len(paths) > 0 && paths[0] != "" {
		return paths[0]
	}
	pwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return pwd
}

type WorkspaceRegistry struct {
	Workspaces map[string]WorkspaceEntry `json:"workspaces"`
}

type WorkspaceEntry struct {
	ConversationID string    `json:"conversation_id"`
	LastSeen       time.Time `json:"last_seen"`
}

// RegisterWorkspaceConversation tracks conversationId -> workspace mapping and detects concurrency conflicts (HK-4).
func RegisterWorkspaceConversation(ws, convID string) (string, bool) {
	if ws == "" || convID == "" {
		return "", false
	}
	regPath := filepath.Join(hooksStateDir(), "workspaces.json")
	var reg WorkspaceRegistry
	reg.Workspaces = make(map[string]WorkspaceEntry)

	data, err := os.ReadFile(regPath)
	if err == nil {
		_ = json.Unmarshal(data, &reg)
	}
	if reg.Workspaces == nil {
		reg.Workspaces = make(map[string]WorkspaceEntry)
	}

	prevConv := ""
	conflict := false

	if entry, ok := reg.Workspaces[ws]; ok {
		if entry.ConversationID != "" && entry.ConversationID != convID && time.Since(entry.LastSeen) < 12*time.Hour {
			prevConv = entry.ConversationID
			conflict = true
		}
	}

	reg.Workspaces[ws] = WorkspaceEntry{
		ConversationID: convID,
		LastSeen:       time.Now(),
	}

	outData, err := json.MarshalIndent(reg, "", "  ")
	if err == nil {
		_ = os.WriteFile(regPath, outData, 0600)
	}

	return prevConv, conflict
}

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
		sb.WriteString(fmt.Sprintf("• Objetivo: %s\n", summary.Objective))
	}
	if summary.Status != "" {
		sb.WriteString(fmt.Sprintf("• Status/Fase: %s\n", summary.Status))
	}

	// HK-4: Check concurrent conversation conflict in active workspace
	if in.ConversationID != "" {
		if prevConv, conflict := RegisterWorkspaceConversation(ws, in.ConversationID); conflict {
			sb.WriteString(fmt.Sprintf("⚠️ [agyo] Conflito de sessão detectado: outra conversa (%s) esteve ativa recentemente neste workspace. A memória operacional (.agents/session/) é compartilhada.\n", prevConv))
		}
	}

	// Read active todo.md to collect In Progress tasks
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	inProgressTasks := extractInProgressTasks(todoPath)
	if len(inProgressTasks) > 0 {
		sb.WriteString("• Tarefas em Progresso:\n")
		for _, t := range inProgressTasks {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}

	// HK-5: Suggest compaction if session footprint is in yellow or red zone, or done tasks >= 5
	if summary.HealthStatus == "Moderate 🟡" || summary.HealthStatus == "Bloated 🔴" || summary.DoneTasks >= 5 {
		sb.WriteString(fmt.Sprintf("💡 [agyo] Pegada de sessão em zona moderada/crítica (%d tarefas concluídas, ~%d tokens, %s). Execute 'agyo session compact' para evitar context bloat.\n", summary.DoneTasks, summary.EstimatedTokens, summary.HealthStatus))
	}

	// HK-8: Observe & inject Multi-Profile Chrome map when multiple authenticated profiles exist
	if pInfo, pErr := platform.Detect(); pErr == nil {
		profs, _ := profile.DiscoverProfiles(pInfo.BrowserProfile, "isolated")
		if len(profs) == 0 {
			profs, _ = profile.DiscoverProfiles(pInfo.SystemChromeDir, "system")
		}
		var authLabels []string
		for _, p := range profs {
			if p.Kind == profile.ProfileKindGhost {
				continue
			}
			if p.HostedDomain != "" {
				authLabels = append(authLabels, fmt.Sprintf("%s (%s: %s)", p.Directory, p.Kind, p.HostedDomain))
			} else {
				authLabels = append(authLabels, fmt.Sprintf("%s (%s)", p.Directory, p.Kind))
			}
		}
		if len(authLabels) > 1 {
			sb.WriteString(fmt.Sprintf("🧭 [agyo] Multi-Profile Chrome ativo: %s. Use 'agyo browser open <url>' ou 'agyo browser resolve <url>' para roteamento automático sem abrir janelas fantasmas.\n", strings.Join(authLabels, ", ")))
		}
	}

	msg := sb.String()
	if len(msg) > MaxPromptSummary {
		msg = msg[:MaxPromptSummary-120] + "\n...(resumo truncado)\n⚠️ Memória operacional extensa (>1.5 KB). Execute 'agyo session compact' para poupar tokens de contexto.\n"
	}

	out.InjectSteps = append(out.InjectSteps, StepInjection{
		EphemeralMessage: msg,
	})

	return out, nil
}

var codeExtRegex = regexp.MustCompile(`\.(go|ts|tsx|js|jsx|py|rs|rb|php|java|c|cpp|h|hpp|cs)$`)
var testBuildCommandRegex = regexp.MustCompile(`(?i)\b(go test|make test|npm test|pytest|cargo test|yarn test|pnpm test|vitest|go build|tsc|make build)\b`)
var destructiveCommandRegex = regexp.MustCompile(`(?i)\b(rm\s+-[a-zA-Z]*r[a-zA-Z]*f|git\s+reset\s+--hard|git\s+clean\s+-[a-zA-Z]*f|drop\s+database|drop\s+table|truncate\s+table)\b`)
var rogueBrowserSpawnRegex = regexp.MustCompile(`(?i)(open\s+.*(-a\s+["']?(Google Chrome|Chrome)["']?|-b\s+com\.google\.Chrome)|(google-chrome|chromium(-browser)?|Google\s+Chrome\.app\/Contents\/MacOS\/Google\s+Chrome)|--profile-directory=|osascript[\s\S]*(Google Chrome|Chrome)[\s\S]*(make\s+new\s+(tab|window)|click\s+menu\s+item|open\s+location))`)

func isWriteFileTool(toolName string) bool {
	return toolName == "write_to_file" ||
		toolName == "replace_file_content" ||
		toolName == "write_file" ||
		toolName == "edit_file" ||
		strings.HasSuffix(toolName, "_write_file") ||
		strings.HasSuffix(toolName, "_edit_file")
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
		targetFile := extractTargetFilePath(in.ToolCall.Args)
		if targetFile != "" && codeExtRegex.MatchString(targetFile) {
			ws := TargetWorkspace(in.WorkspacePaths)
			st := LoadConversationState(in.ConversationID, ws)
			st.DirtyCode = true
			_ = st.Save()
		}
	}

	if toolName == "run_command" || toolName == "execute_bash" || toolName == "shell" {
		cmdStr := extractCommandLine(in.ToolCall.Args)

		// HK-7: Block rogue Chrome window/tab/profile spawning commands
		if rogueBrowserSpawnRegex.MatchString(cmdStr) {
			return &PreToolUseOutput{
				Decision: "deny",
				Reason:   "Bloqueado pelo guardrail de Instância Única e Multi-Profile do agyo: é proibido abrir janelas paralelas ('open -a Google Chrome'), criar abas avulsas ('make new tab') ou alternar perfis via menu AppleScript. Use 'agyo browser profiles' e 'agyo browser resolve <url>' para descobrir o perfil autenticado correto, 'agyo browser start --profile <seletor>' para o Chrome isolado (CDP 9222), ou reutilize estritamente a aba ativa existente.",
			}, nil
		}

		// HK-6: Automated checkpoint before destructive commands
		if destructiveCommandRegex.MatchString(cmdStr) {
			ws := TargetWorkspace(in.WorkspacePaths)
			_, err := checkpoint.Create(ws, "pre-destructive", fmt.Sprintf("Automatic safety checkpoint before: %s", cmdStr))
			if err != nil {
				LogHookError("pretooluse-checkpoint", err)
			}
		}
	}

	return &PreToolUseOutput{Decision: "allow"}, nil
}

// HandlePostToolUse implements HK-3: tracking code edits vs verification runs per conversation.
func HandlePostToolUse(ctx context.Context, in *PostToolUseInput) (*PostToolUseOutput, error) {
	out := &PostToolUseOutput{}
	ws := TargetWorkspace(in.WorkspacePaths)
	st := LoadConversationState(in.ConversationID, ws)

	if in.ToolCall != nil {
		toolName := strings.ToLower(in.ToolCall.Name)

		// 1. Check if tool wrote or modified code
		if isWriteFileTool(toolName) {
			targetFile := extractTargetFilePath(in.ToolCall.Args)
			if targetFile != "" && codeExtRegex.MatchString(targetFile) {
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
	return out, nil
}

// HandleStop implements HK-2: blocking agent stop when tasks are in progress or code is unverified.
func HandleStop(ctx context.Context, in *StopInput) (*StopOutput, error) {
	out := &StopOutput{}

	// Only gate when the model explicitly requested stop
	if in.TerminationReason != "model_stop" {
		return out, nil
	}

	ws := TargetWorkspace(in.WorkspacePaths)
	st := LoadConversationState(in.ConversationID, ws)

	// Anti-loop protection: allow stop if threshold reached
	if st.StopBlocks >= MaxStopBlocks {
		return out, nil
	}

	// Check 1: In-progress tasks in todo.md
	todoPath := filepath.Join(ws, ".agents", "session", "todo.md")
	inProgress := extractInProgressTasks(todoPath)
	if len(inProgress) > 0 {
		st.StopBlocks++
		_ = st.Save()
		return &StopOutput{
			Decision: "continue",
			Reason:   fmt.Sprintf("O arquivo .agents/session/todo.md possui %d tarefa(s) marcadas 'Em Progresso' ([ ]). Conclua a tarefa e marque [x], ou desmarque antes de finalizar: %s", len(inProgress), strings.Join(inProgress, "; ")),
		}, nil
	}

	// Check 2: Unverified code edits
	if st.DirtyCode {
		st.StopBlocks++
		_ = st.Save()
		return &StopOutput{
			Decision: "continue",
			Reason:   "Arquivos de código foram modificados nesta sessão sem execução de testes de verificação ou compilação de sucesso (ex: go test, npm test, pytest, make build). Execute as verificações antes de encerrar.",
		}, nil
	}

	return out, nil
}

func extractTargetFilePath(args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	for _, key := range []string{"TargetFile", "targetFile", "path", "file_path", "filePath"} {
		if val, ok := args[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

func extractCommandLine(args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	for _, key := range []string{"CommandLine", "commandLine", "command", "cmd"} {
		if val, ok := args[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

func extractInProgressTasks(todoPath string) []string {
	data, err := os.ReadFile(todoPath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var tasks []string
	inProgressSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Em Progresso") {
			inProgressSection = true
			continue
		} else if strings.HasPrefix(trimmed, "## ") {
			inProgressSection = false
		}

		if inProgressSection {
			if strings.HasPrefix(trimmed, "- [ ]") {
				task := strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ]"))
				if task != "" {
					tasks = append(tasks, task)
				}
			}
		}
	}
	return tasks
}

// Config file management for Antigravity hooks.json

type AntigravityHookGroup struct {
	Matcher string                   `json:"matcher,omitempty"`
	Hooks   []AntigravityHookCommand `json:"hooks"`
}

type AntigravityHookCommand struct {
	Type    string `json:"type,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type AntigravityNamedHook struct {
	Enabled       bool                     `json:"enabled"`
	PreInvocation []AntigravityHookCommand `json:"PreInvocation,omitempty"`
	PreToolUse    []AntigravityHookGroup   `json:"PreToolUse,omitempty"`
	PostToolUse   []AntigravityHookGroup   `json:"PostToolUse,omitempty"`
	Stop          []AntigravityHookCommand `json:"Stop,omitempty"`
}

// DefaultAgyoHookSpec returns the canonical agyo-session-agent hook configuration.
func DefaultAgyoHookSpec() AntigravityNamedHook {
	return AntigravityNamedHook{
		Enabled: true,
		PreInvocation: []AntigravityHookCommand{
			{
				Type:    "command",
				Command: "agyo hook run PreInvocation",
				Timeout: 5,
			},
		},
		PreToolUse: []AntigravityHookGroup{
			{
				Matcher: "run_command|write_to_file|replace_file_content",
				Hooks: []AntigravityHookCommand{
					{
						Type:    "command",
						Command: "agyo hook run PreToolUse",
						Timeout: 5,
					},
				},
			},
		},
		PostToolUse: []AntigravityHookGroup{
			{
				Matcher: "write_to_file|replace_file_content|run_command",
				Hooks: []AntigravityHookCommand{
					{
						Type:    "command",
						Command: "agyo hook run PostToolUse",
						Timeout: 5,
					},
				},
			},
		},
		Stop: []AntigravityHookCommand{
			{
				Type:    "command",
				Command: "agyo hook run Stop",
				Timeout: 5,
			},
		},
	}
}

// ResolveHooksConfigPath finds the target hooks.json for user or workspace scope.
func ResolveHooksConfigPath(scope string, workspace string) (string, error) {
	if scope == "workspace" {
		if workspace == "" {
			workspace = "."
		}
		return filepath.Join(workspace, ".agents", "hooks.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Prefer ~/.gemini/config/hooks.json if ~/.gemini/config exists, else ~/.gemini/antigravity/hooks.json
	configDir := filepath.Join(home, ".gemini", "config")
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		return filepath.Join(configDir, "hooks.json"), nil
	}
	return filepath.Join(home, ".gemini", "antigravity", "hooks.json"), nil
}

// InstallAntigravityHook merges agyo-session-agent into target hooks.json, creating backups.
func InstallAntigravityHook(scope string, workspace string, dryRun bool) (string, string, error) {
	targetFile, err := ResolveHooksConfigPath(scope, workspace)
	if err != nil {
		return "", "", err
	}

	var root map[string]interface{}
	data, err := os.ReadFile(targetFile)
	if err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return "", "", fmt.Errorf("corrupt hooks.json at %s: %w", targetFile, err)
		}
	} else {
		root = make(map[string]interface{})
	}

	agyoSpec := DefaultAgyoHookSpec()
	root[AgyoHookName] = agyoSpec

	newData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", "", err
	}
	newData = append(newData, '\n')

	if dryRun {
		return targetFile, string(newData), nil
	}

	_ = os.MkdirAll(filepath.Dir(targetFile), 0755)

	// Create backup if targetFile already existed
	if len(data) > 0 {
		backupPath := fmt.Sprintf("%s.bak-%s", targetFile, time.Now().Format("20060102-150405"))
		_ = os.WriteFile(backupPath, data, 0644)
	}

	if err := os.WriteFile(targetFile, newData, 0644); err != nil {
		return "", "", err
	}

	return targetFile, string(newData), nil
}

// UninstallAntigravityHook removes agyo-session-agent from hooks.json.
func UninstallAntigravityHook(scope string, workspace string) (string, error) {
	targetFile, err := ResolveHooksConfigPath(scope, workspace)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(targetFile)
	if os.IsNotExist(err) {
		return targetFile, nil
	}
	if err != nil {
		return "", err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", err
	}

	if _, exists := root[AgyoHookName]; !exists {
		return targetFile, nil
	}

	delete(root, AgyoHookName)
	newData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	newData = append(newData, '\n')

	return targetFile, os.WriteFile(targetFile, newData, 0644)
}

// AntigravityHookStatus checks if agyo-session-agent is configured and enabled in hooks.json.
func AntigravityHookStatus(scope string, workspace string) (bool, string, error) {
	targetFile, err := ResolveHooksConfigPath(scope, workspace)
	if err != nil {
		return false, "", err
	}
	data, err := os.ReadFile(targetFile)
	if os.IsNotExist(err) {
		return false, targetFile, nil
	}
	if err != nil {
		return false, targetFile, err
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false, targetFile, err
	}

	raw, exists := root[AgyoHookName]
	if !exists {
		return false, targetFile, nil
	}

	var named struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &named); err == nil && named.Enabled {
		return true, targetFile, nil
	}
	return false, targetFile, nil
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
		var in PreInvocationInput
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				LogHookError(event, fmt.Errorf("unmarshal error: %w", err))
			}
		}
		out, handleErr := HandlePreInvocation(ctx, &in)
		if handleErr != nil {
			LogHookError(event, handleErr)
			out = &PreInvocationOutput{}
		}
		res, _ := json.Marshal(out)
		_, _ = w.Write(append(res, '\n'))

	case "pretooluse":
		var in PreToolUseInput
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				LogHookError(event, fmt.Errorf("unmarshal error: %w", err))
			}
		}
		out, handleErr := HandlePreToolUse(ctx, &in)
		if handleErr != nil {
			LogHookError(event, handleErr)
			out = &PreToolUseOutput{Decision: "allow"}
		}
		res, _ := json.Marshal(out)
		_, _ = w.Write(append(res, '\n'))

	case "posttooluse":
		var in PostToolUseInput
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				LogHookError(event, fmt.Errorf("unmarshal error: %w", err))
			}
		}
		out, handleErr := HandlePostToolUse(ctx, &in)
		if handleErr != nil {
			LogHookError(event, handleErr)
			out = &PostToolUseOutput{}
		}
		res, _ := json.Marshal(out)
		_, _ = w.Write(append(res, '\n'))

	case "stop":
		var in StopInput
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				LogHookError(event, fmt.Errorf("unmarshal error: %w", err))
			}
		}
		out, handleErr := HandleStop(ctx, &in)
		if handleErr != nil {
			LogHookError(event, handleErr)
			out = &StopOutput{}
		}
		res, _ := json.Marshal(out)
		_, _ = w.Write(append(res, '\n'))

	default:
		LogHookError(event, fmt.Errorf("unknown hook event: %s", event))
		_, _ = io.WriteString(w, "{}\n")
	}

	return nil
}
