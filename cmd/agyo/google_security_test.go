package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

// captureOutput captures stdout and stderr during function execution.
func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	stderr := os.Stderr
	os.Stdout = w
	os.Stderr = w
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()

	f()
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestCLI_GoogleAuth_CredentialsMasking ensures credentials.json import works and redacts secrets (PLUG-04).
func TestCLI_GoogleAuth_CredentialsMasking(t *testing.T) {
	tempDir := t.TempDir()
	geminiDir := filepath.Join(tempDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0700)

	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: geminiDir,
	}

	credsPath := filepath.Join(tempDir, "credentials.json")
	secret := "GOCSPX-supersecretkey999"
	payload := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"project_id": "test-project",
			"client_secret": "` + secret + `"
		}
	}`
	if err := os.WriteFile(credsPath, []byte(payload), 0600); err != nil {
		t.Fatalf("failed to write test credentials: %v", err)
	}

	output := captureOutput(func() {
		origExit := exitFunc
		defer func() { exitFunc = origExit }()
		exitFunc = func(code int) {}

		// Pass port 99999 so it terminates cleanly after credentials parsing
		runGoogle(info, []string{"auth", "--credentials", credsPath, "--port", "99999"})
	})

	// 1. Verify that raw secret was NEVER printed in stdout or stderr
	if strings.Contains(output, secret) {
		t.Fatalf("CRITICAL SECURITY LEAK: raw client_secret found in CLI output:\n%s", output)
	}

	// 2. Verify masked secret is displayed safely
	if !strings.Contains(output, "Client Secret : GOC...999") {
		t.Fatalf("expected masked secret in CLI output, got:\n%s", output)
	}

	// 3. Verify client ID was printed
	if !strings.Contains(output, "test-client-id.apps.googleusercontent.com") {
		t.Fatalf("expected client_id in CLI output, got:\n%s", output)
	}
}

// TestCLI_GoogleCalendar_ICS_Security verifies path traversal prevention and CRLF sanitization in CLI (PLUG-03).
func TestCLI_GoogleCalendar_ICS_Security(t *testing.T) {
	tempDir := t.TempDir()
	geminiDir := filepath.Join(tempDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0700)

	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: geminiDir,
	}

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	exitFunc = func(code int) {}

	t.Run("Valid ICS export with CRLF sanitization", func(t *testing.T) {
		outICS := "test_cli_clean_event.ics"
		defer os.Remove(outICS)

		output := captureOutput(func() {
			runGoogle(info, []string{
				"calendar", "ics",
				"Reunião com CRLF\r\nSUMMARY:HACK\r\nATTENDEE:bad@evil.com",
				"--desc", "Notas de teste\r\nTRIGGER:pwn",
				"--loc", "Sala 101",
				"--out", outICS,
			})
		})

		if !strings.Contains(output, "Arquivo iCalendar exportado com sucesso") {
			t.Fatalf("expected successful ICS export, got: %s", output)
		}

		data, err := os.ReadFile(outICS)
		if err != nil {
			t.Fatalf("failed to read exported ICS file: %v", err)
		}

		content := string(data)
		lines := strings.Split(content, "\r\n")
		summaryCount := 0
		attendeeCount := 0
		for _, l := range lines {
			if strings.HasPrefix(l, "SUMMARY:") {
				summaryCount++
			}
			if strings.HasPrefix(l, "ATTENDEE:") {
				attendeeCount++
			}
		}

		if summaryCount != 1 {
			t.Errorf("CRLF injection detected in file: expected 1 SUMMARY line, found %d", summaryCount)
		}
		if attendeeCount != 0 {
			t.Errorf("CRLF injection detected: found injected ATTENDEE line in %s", content)
		}
	})

	t.Run("Path traversal in --out rejected", func(t *testing.T) {
		output := captureOutput(func() {
			runGoogle(info, []string{
				"calendar", "ics",
				"Escape Attempt",
				"--out", "../../../escaped.ics",
			})
		})

		if !strings.Contains(output, "path traversal detectado") && !strings.Contains(output, "fora do diretório autorizado") {
			t.Fatalf("expected path traversal error in output, got: %s", output)
		}
	})
}

// TestCLI_GoogleCalendar_WebZeroAuthURL verifies web URL generation with strict query escaping (PLUG-02).
func TestCLI_GoogleCalendar_WebZeroAuthURL(t *testing.T) {
	tempDir := t.TempDir()
	geminiDir := filepath.Join(tempDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0700)

	info := &platform.Info{
		OS:        "darwin",
		GeminiDir: geminiDir,
	}

	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	exitFunc = func(code int) {}

	output := captureOutput(func() {
		runGoogle(info, []string{
			"calendar", "add",
			"Sync com Time & Parceiros",
			"--desc", "Pauta: arquitetura & infraestrutura",
			"--loc", "Meet: https://meet.google.com/xyz",
			"--web",
		})
	})

	if !strings.Contains(output, "Agendamento Web do Google Calendar (Zero-Auth)") {
		t.Fatalf("expected web zero-auth header, got: %s", output)
	}

	if !strings.Contains(output, "action=TEMPLATE") {
		t.Fatalf("expected action=TEMPLATE in URL, got: %s", output)
	}

	// Verify that & in title was escaped as %26
	if !strings.Contains(output, "Sync+com+Time+%26+Parceiros") {
		t.Fatalf("expected escaped title in URL, got: %s", output)
	}
}
