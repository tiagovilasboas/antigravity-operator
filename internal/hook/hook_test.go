package hook_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/hook"
)

func TestHookInstallAndUninstall(t *testing.T) {
	tempDir := t.TempDir()
	gitHooksDir := filepath.Join(tempDir, ".git", "hooks")
	if err := os.MkdirAll(gitHooksDir, 0755); err != nil {
		t.Fatalf("failed to create fake .git/hooks: %v", err)
	}

	// 1. Install
	hookPath, err := hook.Install(tempDir)
	if err != nil {
		t.Fatalf("failed to install hook: %v", err)
	}

	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("expected hook file to exist: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("expected hook to be executable, got mode: %v", info.Mode())
	}

	// 2. Uninstall
	if err := hook.Uninstall(tempDir); err != nil {
		t.Fatalf("failed to uninstall hook: %v", err)
	}

	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Errorf("expected hook file to be removed")
	}

	// 3. Uninstall when already absent (idempotent)
	if err := hook.Uninstall(tempDir); err != nil {
		t.Errorf("expected idempotent uninstall, got error: %v", err)
	}
}

func TestHookInstall_NotAGitRepo(t *testing.T) {
	tempDir := t.TempDir() // no .git
	_, err := hook.Install(tempDir)
	if err == nil {
		t.Error("expected error installing in non-git directory, got nil")
	}
}
