package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerate_Bash(t *testing.T) {
	var buf bytes.Buffer
	err := Generate("bash", &buf)
	if err != nil {
		t.Fatalf("unexpected error for bash: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "_agyo_completion") || !strings.Contains(out, "complete -F _agyo_completion agyo") {
		t.Errorf("bash completion script missing expected definitions: %s", out)
	}
}

func TestGenerate_Zsh(t *testing.T) {
	var buf bytes.Buffer
	err := Generate("zsh", &buf)
	if err != nil {
		t.Fatalf("unexpected error for zsh: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "#compdef agyo") || !strings.Contains(out, "_agyo") {
		t.Errorf("zsh completion script missing expected definitions: %s", out)
	}
}

func TestGenerate_Fish(t *testing.T) {
	var buf bytes.Buffer
	err := Generate("fish", &buf)
	if err != nil {
		t.Fatalf("unexpected error for fish: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "complete -c agyo") {
		t.Errorf("fish completion script missing expected definitions: %s", out)
	}
}

func TestGenerate_InvalidShell(t *testing.T) {
	var buf bytes.Buffer
	err := Generate("powershell", &buf)
	if err == nil {
		t.Errorf("expected error for unsupported shell, got nil")
	}
}

func TestGenerate_SyncUpdateMCPFlag(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		var buf bytes.Buffer
		if err := Generate(sh, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "update-mcp") {
			t.Errorf("%s completion does not offer sync --update-mcp", sh)
		}
	}
}
