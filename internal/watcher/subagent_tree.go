package watcher

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

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
