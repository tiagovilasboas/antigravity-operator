package profile_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

func TestPIDManagement(t *testing.T) {
	tempDir := t.TempDir()

	// Initially no PID
	if pid := profile.ReadPID(tempDir); pid != 0 {
		t.Errorf("expected PID 0, got: %d", pid)
	}

	// Write PID
	testPID := 12345
	if err := profile.SavePID(tempDir, testPID); err != nil {
		t.Fatalf("failed to save PID: %v", err)
	}

	// Read PID
	read := profile.ReadPID(tempDir)
	if read != testPID {
		t.Errorf("expected PID %d, got: %d", testPID, read)
	}

	// Corrupted PID file
	pidFile := filepath.Join(tempDir, profile.PIDFileName)
	_ = os.WriteFile(pidFile, []byte("not-a-number"), 0644)
	if corrupted := profile.ReadPID(tempDir); corrupted != 0 {
		t.Errorf("expected PID 0 for corrupted file, got: %d", corrupted)
	}
}

func TestCheckStatusInactivePort(t *testing.T) {
	tempDir := t.TempDir()

	// Port 59998 inactive
	st := profile.CheckStatusOnPort(tempDir, 59998)
	if st.IsRunning {
		t.Errorf("expected port 59998 inactive")
	}
	if st.Port != 59998 {
		t.Errorf("expected port 59998, got %d", st.Port)
	}
}

func TestCheckStatusActivePort_Mock(t *testing.T) {
	tempDir := t.TempDir()

	// Mock Chrome /json/version endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(profile.ChromeVersionResponse{
				Browser: "Chrome/125.0.0.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Extract port from server.URL (http://127.0.0.1:xxxxx)
	parts := strings.Split(server.URL, ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("failed to parse mock server port: %v", err)
	}

	st := profile.CheckStatusOnPort(tempDir, port)
	if !st.IsRunning {
		t.Error("expected status to be running for mock server")
	}
	if st.Version != "Chrome/125.0.0.0" {
		t.Errorf("expected Chrome/125.0.0.0, got %s", st.Version)
	}
}

func TestStart_MissingChromeBinary(t *testing.T) {
	info := &platform.Info{
		ChromeBin:      "",
		BrowserProfile: t.TempDir(),
	}

	err := profile.Start(info, profile.StartOptions{Port: 9222})
	if err == nil {
		t.Error("expected error for missing Chrome binary, got nil")
	}
}

func TestStop_NoPIDAndDeadPort(t *testing.T) {
	tempDir := t.TempDir()
	info := &platform.Info{
		BrowserProfile: tempDir,
	}

	// Idempotent stop: if nothing is running, Stop should return nil gracefully
	err := profile.Stop(info)
	// If port 9222 is active on host without PID, it returns an error; if inactive, it returns nil.
	// Either way, it must not panic.
	_ = err
}

func TestStop_DeadPID(t *testing.T) {
	tempDir := t.TempDir()
	info := &platform.Info{
		BrowserProfile: tempDir,
	}

	// Save a PID of an impossible process (e.g. 999999)
	_ = profile.SavePID(tempDir, 999999)

	err := profile.Stop(info)
	// Stop should handle dead process gracefully and clean up the PID file
	if err != nil {
		t.Logf("Stop returned expected notice for dead PID: %v", err)
	}

	// PID file should be removed
	if pid := profile.ReadPID(tempDir); pid != 0 {
		t.Errorf("expected PID file to be cleaned up, but got %d", pid)
	}
}
