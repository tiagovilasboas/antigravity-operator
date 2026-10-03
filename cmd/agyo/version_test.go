package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersion_InjectedViaLdflags builds the CLI the way the release workflow
// does and checks that -X main.Version reaches `agyo version`.
func TestVersion_InjectedViaLdflags(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not in PATH")
	}
	bin := filepath.Join(t.TempDir(), "agyo")
	build := exec.Command(goBin, "build", "-ldflags", "-X main.Version=9.8.7-test", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("agyo version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "v9.8.7-test") {
		t.Fatalf("version not injected, got: %s", out)
	}
}

func TestVersion_DefaultsToDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("unflagged builds should report dev, got %q", Version)
	}
}
