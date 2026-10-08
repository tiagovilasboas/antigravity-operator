package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ToolCall representa uma ferramenta invocada pelo modelo.
type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Event representa um registro estruturado da transcrição (transcript.jsonl).
type Event struct {
	StepIndex  int        `json:"step_index"`
	Source     string     `json:"source"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	CreatedAt  string     `json:"created_at"`
	Content    string     `json:"content"`
	Thinking   string     `json:"thinking"`
	ToolCalls  []ToolCall `json:"tool_calls"`
	IsQuestion bool       `json:"-"`
	IsDone     bool       `json:"-"`
}

// TranscriptInfo contém metadados de uma sessão descoberta no disco.
type TranscriptInfo struct {
	Path           string
	ConversationID string
	ModTime        time.Time
	Size           int64
}

// AppDataDirs lists the Antigravity app data directories under home whose
// brain/<conversation-id>/.system_generated/logs/transcript.jsonl hold session
// transcripts: ~/.gemini/antigravity (Antigravity 2.0), ~/.gemini/antigravity-cli
// (CLI) and ~/.gemini/antigravity-ide (IDE), per the transcriptPath field in
// https://antigravity.google/docs/hooks/. It is the single source of truth for
// where agyo looks for transcripts.
func AppDataDirs(home string) []string {
	return []string{
		filepath.Join(home, ".gemini", "antigravity"),
		filepath.Join(home, ".gemini", "antigravity-cli"),
		filepath.Join(home, ".gemini", "antigravity-ide"),
	}
}

// FindLatestTranscript returns the most recently modified transcript across
// every directory in AppDataDirs(home).
func FindLatestTranscript(home string) (*TranscriptInfo, error) {
	var latest *TranscriptInfo
	dirs := AppDataDirs(home)
	for _, dir := range dirs {
		t, err := FindActiveTranscript(dir)
		if err != nil {
			continue
		}
		if latest == nil || t.ModTime.After(latest.ModTime) {
			latest = t
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("nenhum transcript ativo encontrado em %s", strings.Join(dirs, ", "))
	}
	return latest, nil
}

// FindActiveTranscript localiza a sessão mais recente em <appDataDir>/brain/*/transcript.jsonl.
// Prefer FindLatestTranscript, which searches every Antigravity app data dir.
func FindActiveTranscript(geminiDir string) (*TranscriptInfo, error) {
	brainDir := filepath.Join(geminiDir, "brain")
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler pasta brain: %w", err)
	}

	var latest *TranscriptInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(brainDir, entry.Name(), ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(candidate)
		if err == nil {
			if latest == nil || info.ModTime().After(latest.ModTime) {
				latest = &TranscriptInfo{
					Path:           candidate,
					ConversationID: entry.Name(),
					ModTime:        info.ModTime(),
					Size:           info.Size(),
				}
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("nenhum transcript ativo encontrado em %s", brainDir)
	}

	return latest, nil
}

// ParseLine decodifica uma linha JSONL em um Event estruturado.
func ParseLine(line []byte) (*Event, error) {
	clean := strings.TrimSpace(string(line))
	if len(clean) == 0 {
		return nil, fmt.Errorf("linha vazia")
	}

	var evt Event
	if err := json.Unmarshal([]byte(clean), &evt); err != nil {
		return nil, err
	}

	for _, call := range evt.ToolCalls {
		if call.Name == "ask_question" {
			evt.IsQuestion = true
		}
	}
	if evt.Status == "DONE" {
		evt.IsDone = true
	}

	return &evt, nil
}

// Summary extrai uma descrição compacta e legível do evento para exibição no terminal.
func (e *Event) Summary() string {
	var sb strings.Builder

	switch e.Type {
	case "USER_INPUT":
		content := strings.TrimSpace(e.Content)
		if len(content) > 100 {
			content = content[:97] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")
		sb.WriteString(fmt.Sprintf("👤 [User #%d] %s", e.StepIndex, content))

	case "PLANNER_RESPONSE":
		if e.Thinking != "" {
			thinking := strings.TrimSpace(e.Thinking)
			if len(thinking) > 120 {
				thinking = thinking[:117] + "..."
			}
			thinking = strings.ReplaceAll(thinking, "\n", " ")
			sb.WriteString(fmt.Sprintf("💭 [Think #%d] %s\n", e.StepIndex, thinking))
		}
		if len(e.ToolCalls) > 0 {
			for i, tc := range e.ToolCalls {
				if i > 0 {
					sb.WriteString("\n")
				}
				if tc.Name == "ask_question" {
					sb.WriteString(fmt.Sprintf("🔔 [INTERAÇÃO #%d] O agente precisa da sua resposta!", e.StepIndex))
				} else if tc.Name == "invoke_subagent" {
					subs := parseInvokeSubagents(tc.Args)
					if len(subs) > 0 {
						var subStrs []string
						for _, sub := range subs {
							promptSummary := strings.TrimSpace(sub.Prompt)
							if len(promptSummary) > 80 {
								promptSummary = promptSummary[:77] + "..."
							}
							promptSummary = strings.ReplaceAll(promptSummary, "\n", " ")
							subStr := fmt.Sprintf("🤖 [SUBAGENT SPAWNED] Role: %q (type: %s)\n   ↳ Task: %q", sub.Role, sub.TypeName, promptSummary)
							subStrs = append(subStrs, subStr)
						}
						sb.WriteString(strings.Join(subStrs, "\n"))
					} else {
						argsSummary := extractArgsSummary(tc.Name, tc.Args)
						sb.WriteString(fmt.Sprintf("🛠️  [Tool #%d] %s(%s)", e.StepIndex, tc.Name, argsSummary))
					}
				} else if tc.Name == "send_message" {
					msgParam := parseSendMessage(tc.Args)
					if msgParam.Recipient != "" || msgParam.Message != "" {
						msgSummary := strings.TrimSpace(msgParam.Message)
						if len(msgSummary) > 80 {
							msgSummary = msgSummary[:77] + "..."
						}
						msgSummary = strings.ReplaceAll(msgSummary, "\n", " ")
						sb.WriteString(fmt.Sprintf("💬 [AGENT MESSAGE] To: %s | %s", msgParam.Recipient, msgSummary))
					} else {
						argsSummary := extractArgsSummary(tc.Name, tc.Args)
						sb.WriteString(fmt.Sprintf("🛠️  [Tool #%d] %s(%s)", e.StepIndex, tc.Name, argsSummary))
					}
				} else {
					argsSummary := extractArgsSummary(tc.Name, tc.Args)
					sb.WriteString(fmt.Sprintf("🛠️  [Tool #%d] %s(%s)", e.StepIndex, tc.Name, argsSummary))
				}
			}
		} else if e.Thinking == "" {
			sb.WriteString(fmt.Sprintf("🤖 [Agent #%d] Planejando próxima ação...", e.StepIndex))
		}

	case "GENERIC":
		content := strings.TrimSpace(e.Content)
		if len(content) > 80 {
			content = content[:77] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")
		if content != "" {
			sb.WriteString(fmt.Sprintf("⚡ [Step #%d] %s", e.StepIndex, content))
		}

	default:
		sb.WriteString(fmt.Sprintf("ℹ️  [%s #%d] Status: %s", e.Type, e.StepIndex, e.Status))
	}

	return sb.String()
}

// MetadataSummary descreve o evento só com tipo, passo, ferramenta e status,
// sem texto de prompt, raciocínio, argumentos ou saída. Use em superfícies
// servidas por HTTP (dashboard), onde o texto da sessão não deve vazar.
func (e *Event) MetadataSummary() string {
	switch e.Type {
	case "USER_INPUT":
		return fmt.Sprintf("👤 [User #%d]", e.StepIndex)
	case "PLANNER_RESPONSE":
		var lines []string
		if e.Thinking != "" {
			lines = append(lines, fmt.Sprintf("💭 [Think #%d]", e.StepIndex))
		}
		for _, tc := range e.ToolCalls {
			if tc.Name == "ask_question" {
				lines = append(lines, fmt.Sprintf("🔔 [INTERAÇÃO #%d] O agente precisa da sua resposta!", e.StepIndex))
			} else {
				lines = append(lines, fmt.Sprintf("🛠️  [Tool #%d] %s", e.StepIndex, tc.Name))
			}
		}
		if len(lines) == 0 {
			return fmt.Sprintf("🤖 [Agent #%d] Planejando próxima ação...", e.StepIndex)
		}
		return strings.Join(lines, "\n")
	case "GENERIC":
		if e.Status == "" {
			return fmt.Sprintf("⚡ [Step #%d]", e.StepIndex)
		}
		return fmt.Sprintf("⚡ [Step #%d] Status: %s", e.StepIndex, e.Status)
	default:
		return fmt.Sprintf("ℹ️  [%s #%d] Status: %s", e.Type, e.StepIndex, e.Status)
	}
}

func extractArgsSummary(toolName string, rawArgs json.RawMessage) string {
	if len(rawArgs) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(rawArgs, &m); err != nil {
		return ""
	}

	switch toolName {
	case "run_command":
		if cmd, ok := m["CommandLine"].(string); ok {
			cmd = strings.TrimSpace(cmd)
			cmd = strings.Trim(cmd, `"`)
			if len(cmd) > 60 {
				return cmd[:57] + "..."
			}
			return cmd
		}
	case "view_file", "replace_file_content":
		if file, ok := m["AbsolutePath"].(string); ok {
			return filepath.Base(strings.Trim(file, `"`))
		}
		if file, ok := m["TargetFile"].(string); ok {
			return filepath.Base(strings.Trim(file, `"`))
		}
	case "write_to_file":
		if file, ok := m["TargetFile"].(string); ok {
			return filepath.Base(strings.Trim(file, `"`))
		}
	case "call_mcp_tool":
		if server, ok := m["ServerName"].(string); ok {
			if tool, ok := m["ToolName"].(string); ok {
				return fmt.Sprintf("%s/%s", server, tool)
			}
		}
	case "invoke_subagent":
		subs := parseInvokeSubagents(rawArgs)
		if len(subs) > 0 {
			return fmt.Sprintf("Role: %s (type: %s)", subs[0].Role, subs[0].TypeName)
		}
	case "send_message":
		msg := parseSendMessage(rawArgs)
		if msg.Recipient != "" {
			return fmt.Sprintf("To: %s", msg.Recipient)
		}
	}

	// Resumo padrão se não mapeado
	for _, v := range m {
		if s, ok := v.(string); ok && len(s) > 0 && len(s) < 40 {
			return s
		}
	}
	return ""
}

// SubagentNode representa um subagente invocado na árvore de execução.
type SubagentNode struct {
	Role      string         `json:"role"`
	TypeName  string         `json:"type_name"`
	Prompt    string         `json:"prompt"`
	Model     string         `json:"model,omitempty"`
	Workspace string         `json:"workspace,omitempty"`
	Children  []SubagentNode `json:"children,omitempty"`
	Messages  []AgentMessage `json:"messages,omitempty"`
}

// AgentMessage representa uma mensagem trocada entre agentes (send_message).
type AgentMessage struct {
	From      string `json:"from,omitempty"`
	To        string `json:"to"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SubagentTree mantém a hierarquia e o estado em memória dos subagentes e mensagens.
type SubagentTree struct {
	mu       sync.RWMutex
	Agents   []SubagentNode `json:"agents"`
	Messages []AgentMessage `json:"messages"`
}

// NewSubagentTree instancia uma nova árvore de subagentes.
func NewSubagentTree() *SubagentTree {
	return &SubagentTree{
		Agents:   make([]SubagentNode, 0),
		Messages: make([]AgentMessage, 0),
	}
}

var globalSubagentTree = NewSubagentTree()

// GetGlobalSubagentTree retorna a árvore global compartilhada de subagentes.
func GetGlobalSubagentTree() *SubagentTree {
	return globalSubagentTree
}

// AddSpawn adiciona um subagente invocado à árvore.
func (t *SubagentTree) AddSpawn(role, typeName, prompt, model, workspace string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Agents = append(t.Agents, SubagentNode{
		Role:      role,
		TypeName:  typeName,
		Prompt:    prompt,
		Model:     model,
		Workspace: workspace,
		Children:  make([]SubagentNode, 0),
		Messages:  make([]AgentMessage, 0),
	})
}

// AddMessage adiciona uma mensagem entre agentes à árvore.
func (t *SubagentTree) AddMessage(to, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	msg := AgentMessage{
		To:      to,
		Message: message,
	}
	t.Messages = append(t.Messages, msg)
	if len(t.Agents) > 0 {
		lastAgent := &t.Agents[len(t.Agents)-1]
		lastAgent.Messages = append(lastAgent.Messages, msg)
	}
}

// GetActiveSubagents retorna uma cópia dos subagentes ativos de forma thread-safe.
func (t *SubagentTree) GetActiveSubagents() []SubagentNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]SubagentNode, len(t.Agents))
	copy(out, t.Agents)
	return out
}

// FormatTree formata visualmente a árvore de subagentes e mensagens.
func (t *SubagentTree) FormatTree() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var sb strings.Builder
	sb.WriteString("=== Active Subagents Tree ===\n")
	if len(t.Agents) == 0 {
		sb.WriteString("  (no active subagents)\n")
	}
	for i, agent := range t.Agents {
		sb.WriteString(fmt.Sprintf("%d. Role: %q (type: %s)\n", i+1, agent.Role, agent.TypeName))
		if agent.Prompt != "" {
			p := strings.TrimSpace(agent.Prompt)
			if len(p) > 80 {
				p = p[:77] + "..."
			}
			p = strings.ReplaceAll(p, "\n", " ")
			sb.WriteString(fmt.Sprintf("   ↳ Task: %q\n", p))
		}
		if len(agent.Messages) > 0 {
			sb.WriteString("   ↳ Messages:\n")
			for _, m := range agent.Messages {
				msgSummary := strings.TrimSpace(m.Message)
				if len(msgSummary) > 60 {
					msgSummary = msgSummary[:57] + "..."
				}
				msgSummary = strings.ReplaceAll(msgSummary, "\n", " ")
				sb.WriteString(fmt.Sprintf("     - To: %s | %s\n", m.To, msgSummary))
			}
		}
	}
	if len(t.Messages) > 0 {
		sb.WriteString("=== Inter-Agent Messages ===\n")
		for _, m := range t.Messages {
			msgSummary := strings.TrimSpace(m.Message)
			if len(msgSummary) > 60 {
				msgSummary = msgSummary[:57] + "..."
			}
			msgSummary = strings.ReplaceAll(msgSummary, "\n", " ")
			sb.WriteString(fmt.Sprintf("💬 To: %s | %s\n", m.To, msgSummary))
		}
	}
	return sb.String()
}

// ProcessEvent processa um evento e atualiza a árvore de subagentes se houver invoke_subagent ou send_message.
func (t *SubagentTree) ProcessEvent(e *Event) {
	if e == nil {
		return
	}
	for _, tc := range e.ToolCalls {
		switch tc.Name {
		case "invoke_subagent":
			subs := parseInvokeSubagents(tc.Args)
			for _, sub := range subs {
				t.AddSpawn(sub.Role, sub.TypeName, sub.Prompt, "", "")
			}
		case "send_message":
			msg := parseSendMessage(tc.Args)
			if msg.Recipient != "" || msg.Message != "" {
				t.AddMessage(msg.Recipient, msg.Message)
			}
		}
	}
}

type SubagentParam struct {
	Role     string
	TypeName string
	Prompt   string
}

func parseInvokeSubagents(rawArgs json.RawMessage) []SubagentParam {
	var result []SubagentParam
	if len(rawArgs) == 0 {
		return result
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(rawArgs, &raw); err != nil {
		return result
	}

	subagentsVal, ok := raw["Subagents"]
	if !ok {
		subagentsVal, ok = raw["subagents"]
	}

	if ok {
		if list, ok := subagentsVal.([]interface{}); ok {
			for _, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					role := getStringField(m, "Role", "role")
					typeName := getStringField(m, "TypeName", "typename", "Type", "type")
					prompt := getStringField(m, "Prompt", "prompt", "Goal", "goal")
					result = append(result, SubagentParam{
						Role:     role,
						TypeName: typeName,
						Prompt:   prompt,
					})
				}
			}
		}
	} else {
		role := getStringField(raw, "Role", "role")
		typeName := getStringField(raw, "TypeName", "typename", "Type", "type")
		prompt := getStringField(raw, "Prompt", "prompt", "Goal", "goal")
		if role != "" || typeName != "" || prompt != "" {
			result = append(result, SubagentParam{
				Role:     role,
				TypeName: typeName,
				Prompt:   prompt,
			})
		}
	}

	return result
}

type SendMessageParam struct {
	Recipient string
	Message   string
}

func parseSendMessage(rawArgs json.RawMessage) SendMessageParam {
	var p SendMessageParam
	if len(rawArgs) == 0 {
		return p
	}
	var m map[string]interface{}
	if err := json.Unmarshal(rawArgs, &m); err != nil {
		return p
	}
	p.Recipient = getStringField(m, "Recipient", "recipient", "RecipientID", "recipient_id")
	p.Message = getStringField(m, "Message", "message", "Content", "content")
	return p
}

func getStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

// NotifyExecutor permite interceptar comandos de notificação no SO (útil para testes ou mocks).
var NotifyExecutor = func(osName, title, message string) {
	switch osName {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "linux":
		_ = exec.Command("notify-send", title, message).Run()
	}
}

// Notify emite notificação visual no SO (macOS/Linux) e alerta sonoro no terminal.
func Notify(osName, title, message string) {
	if os.Getenv("AGYO_DISABLE_NOTIFY") == "1" {
		return
	}
	// Alerta sonoro de terminal (bell)
	fmt.Print("\a")

	NotifyExecutor(osName, title, message)
}

// WatchOptions configura o comportamento do tailing.
type WatchOptions struct {
	Follow       bool
	NotifyOnWait bool
	PollInterval time.Duration
	InitialSteps int
	OSName       string
}

// Stream acompanha o arquivo transcript.jsonl e envia eventos para o handler.
func Stream(ctx context.Context, transcriptPath string, opts WatchOptions, handler func(*Event)) error {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return fmt.Errorf("falha ao abrir transcript: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var allEvents []*Event
	var partial []byte

	// 1. Fase Inicial: lê tudo o que já existe
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if len(partial) > 0 {
				line = append(partial, line...)
				partial = nil
			}
			if line[len(line)-1] == '\n' {
				if evt, pErr := ParseLine(line); pErr == nil {
					allEvents = append(allEvents, evt)
					globalSubagentTree.ProcessEvent(evt)
				}
			} else {
				partial = append(partial, line...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Emite os passos iniciais solicitados (ex: últimos N passos)
	startIdx := 0
	if opts.InitialSteps > 0 && len(allEvents) > opts.InitialSteps {
		startIdx = len(allEvents) - opts.InitialSteps
	}
	for i := startIdx; i < len(allEvents); i++ {
		handler(allEvents[i])
	}

	if !opts.Follow {
		return nil
	}

	poll := opts.PollInterval
	if poll <= 0 {
		poll = 400 * time.Millisecond
	}

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	// 2. Fase de Tailing: escuta novos dados continuamente
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for {
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					if len(partial) > 0 {
						line = append(partial, line...)
						partial = nil
					}
					if line[len(line)-1] == '\n' {
						if evt, pErr := ParseLine(line); pErr == nil {
							globalSubagentTree.ProcessEvent(evt)
							handler(evt)
							if opts.NotifyOnWait && evt.IsQuestion {
								Notify(opts.OSName, "Antigravity Operator", "O agente precisa da sua resposta (pergunta interativa)!")
							}
						}
					} else {
						partial = append(partial, line...)
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
		}
	}
}
