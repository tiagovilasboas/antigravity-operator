package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestRunComputer_Help(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitCode := -1
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { exitFunc = oldExit }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	info := &platform.Info{OS: "darwin", Arch: "arm64"}
	runComputer(info, []string{"--help"})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "agyo computer <subcommand>") {
		t.Errorf("expected computer usage output; got %q", output)
	}
	if exitCalled && exitCode != 0 {
		t.Errorf("unexpected exit code %d", exitCode)
	}
}

func TestRunComputer_UnknownSubcommand(t *testing.T) {
	oldExit := exitFunc
	exitCalled := false
	exitCode := -1
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { exitFunc = oldExit }()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	info := &platform.Info{OS: "darwin", Arch: "arm64"}
	runComputer(info, []string{"unknown_cmd"})

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !exitCalled || exitCode != ExitUsageError {
		t.Errorf("expected exit with ExitUsageError; got called=%v, code=%d", exitCalled, exitCode)
	}
	if !strings.Contains(output, "Unknown computer command") {
		t.Errorf("expected unknown command error; got %q", output)
	}
}
