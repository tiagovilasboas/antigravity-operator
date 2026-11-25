package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
