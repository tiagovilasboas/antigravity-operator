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

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestGoogle_UsageAndHelp(t *testing.T) {
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   t.TempDir(),
		GeminiDir: t.TempDir(),
	}

	out := captureStdout(func() {
		runGoogle(info, []string{"--help"})
	})
	if !strings.Contains(out, "Google Workspace Bridge") {
		t.Errorf("expected usage output, got: %s", out)
	}

	outCal := captureStdout(func() {
		runGoogle(info, []string{"calendar", "--help"})
	})
	if !strings.Contains(outCal, "add") || !strings.Contains(outCal, "ics") {
		t.Errorf("expected calendar actions in help, got: %s", outCal)
	}
}

func TestGoogle_CalendarAdd_ZeroAuthFallback(t *testing.T) {
	tempHome := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: filepath.Join(tempHome, ".gemini", "antigravity"),
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }

	// Agendamento sem autenticacao: deve ativar fallback web zero-auth em vez de falhar
	out := captureStdout(func() {
		runGoogle(info, []string{
			"calendar", "add", "Sessao de Codigo",
			"--start", "hoje 15:00",
			"--end", "+1h",
			"--desc", "Desenvolvimento autonomo com agyo",
		})
	})

	if exitCalled {
		t.Errorf("expected graceful zero-auth fallback without exit, but exit was called")
	}
	if !strings.Contains(out, "Fallback Zero-Auth ativado") && !strings.Contains(out, "Agendamento Web") {
		t.Errorf("expected zero-auth fallback message, got: %s", out)
	}
	if !strings.Contains(out, "https://calendar.google.com/calendar/render?action=TEMPLATE") {
		t.Errorf("expected google calendar render URL, got: %s", out)
	}
	if !strings.Contains(out, "text=Sessao+de+Codigo") && !strings.Contains(out, "text=Sessao") {
		t.Errorf("expected event title in web template URL, got: %s", out)
	}
}

func TestGoogle_CalendarAdd_WithWebFlag(t *testing.T) {
	tempHome := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: filepath.Join(tempHome, ".gemini", "antigravity"),
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitFunc = func(code int) {}

	out := captureStdout(func() {
		runGoogle(info, []string{
			"calendar", "add", "Alinhamento Semanal",
			"--start", "amanha 10:00",
			"--web",
		})
	})

	if !strings.Contains(out, "Agendamento Web do Google Calendar (Zero-Auth)") {
		t.Errorf("expected web scheduling message, got: %s", out)
	}
	if !strings.Contains(out, "https://calendar.google.com/calendar/render?action=TEMPLATE") {
		t.Errorf("expected web template URL, got: %s", out)
	}
}

func TestGoogle_CalendarAdd_ExportICS(t *testing.T) {
	tempHome := t.TempDir()
	icsFile := filepath.Join(tempHome, "reuniao.ics")
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: filepath.Join(tempHome, ".gemini", "antigravity"),
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitFunc = func(code int) {}

	// Passa --ics e --web
	out := captureStdout(func() {
		runGoogle(info, []string{
			"calendar", "add", "Planejamento Estrategico",
			"--start", "2026-10-30 14:00",
			"--end", "+2h",
			"--ics", icsFile,
			"--web",
		})
	})

	if !strings.Contains(out, "exportado com sucesso") {
		t.Errorf("expected ICS export message, got: %s", out)
	}

	data, err := os.ReadFile(icsFile)
	if err != nil {
		t.Fatalf("failed to read exported ICS file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "BEGIN:VCALENDAR") || !strings.Contains(content, "Planejamento Estrategico") {
		t.Errorf("unexpected ICS content: %s", content)
	}
}

func TestGoogle_CalendarICS_Subcommand(t *testing.T) {
	tempHome := t.TempDir()
	outFile := filepath.Join(tempHome, "export.ics")
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: filepath.Join(tempHome, ".gemini", "antigravity"),
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitFunc = func(code int) {}

	out := captureStdout(func() {
		runGoogle(info, []string{
			"calendar", "ics", "Demo de Produto",
			"--start", "hoje 16:00",
			"--end", "+45m",
			"--loc", "Sala 3",
			"--out", outFile,
		})
	})

	if !strings.Contains(out, "exportado com sucesso") {
		t.Errorf("expected ics export confirmation, got: %s", out)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read ics file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "BEGIN:VCALENDAR") || !strings.Contains(content, "Demo de Produto") {
		t.Errorf("unexpected ics content: %s", content)
	}
	if !strings.Contains(content, "LOCATION:Sala 3") {
		t.Errorf("expected location in ics content: %s", content)
	}
}

func TestGoogle_MCP_LifecycleCLI(t *testing.T) {
	tempHome := t.TempDir()
	tempGemini := filepath.Join(tempHome, ".gemini", "antigravity")
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: tempGemini,
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitFunc = func(code int) {}

	// 1. Status inicial (não configurado)
	outStatusInit := captureStdout(func() {
		runGoogle(info, []string{"mcp", "status"})
	})
	if !strings.Contains(outStatusInit, "NÃO CONFIGURADO") {
		t.Errorf("expected NOT CONFIGURED status, got: %s", outStatusInit)
	}

	// 2. Install
	outInstall := captureStdout(func() {
		runGoogle(info, []string{"mcp", "install"})
	})
	if !strings.Contains(outInstall, "instalado e configurado") {
		t.Errorf("expected successful install message, got: %s", outInstall)
	}

	// 3. Status configurado
	outStatus := captureStdout(func() {
		runGoogle(info, []string{"mcp", "status"})
	})
	if !strings.Contains(outStatus, "CONFIGURADO") || !strings.Contains(outStatus, "google-calendar") {
		t.Errorf("expected CONFIGURED status with google-calendar, got: %s", outStatus)
	}

	// 4. Status com --json
	outStatusJSON := captureStdout(func() {
		runGoogle(info, []string{"mcp", "status", "--json"})
	})
	if !strings.Contains(outStatusJSON, `"installed": true`) {
		t.Errorf("expected JSON status with installed: true, got: %s", outStatusJSON)
	}

	// 5. Uninstall
	outUninstall := captureStdout(func() {
		runGoogle(info, []string{"mcp", "uninstall"})
	})
	if !strings.Contains(outUninstall, "desinstalado do Antigravity") {
		t.Errorf("expected uninstall confirmation, got: %s", outUninstall)
	}
}

func TestGoogle_Auth_CredentialsFlag_ErrorHandling(t *testing.T) {
	tempHome := t.TempDir()
	info := &platform.Info{
		OS:        "darwin",
		HomeDir:   tempHome,
		GeminiDir: filepath.Join(tempHome, ".gemini", "antigravity"),
	}

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }

	// Arquivo inexistente: deve emitir erro e não travar
	runGoogle(info, []string{"auth", "--credentials", filepath.Join(tempHome, "nao-existe.json")})
	if !exitCalled {
		t.Errorf("expected exit on non-existent credentials file")
	}
}
