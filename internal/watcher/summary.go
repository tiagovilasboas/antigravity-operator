package watcher

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

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
