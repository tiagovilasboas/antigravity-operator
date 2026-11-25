package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const AgyoHookName = "agyo-session-agent"

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
