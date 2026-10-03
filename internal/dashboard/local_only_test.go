package dashboard

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagoboas/antigravity-operator/internal/platform"
)

// newTestServer binds a free loopback port so the allowlist sees a real port.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	s, err := NewServer(Config{
		Port:         port,
		TargetDir:    t.TempDir(),
		PlatformInfo: &platform.Info{OS: "linux", Arch: "amd64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.listener.Close() })
	return s
}

func serve(s *Server, host, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	return w
}

func TestLocalOnly_HostAndOrigin(t *testing.T) {
	s := newTestServer(t)
	p := strconv.Itoa(s.port)
	other := strconv.Itoa(s.port + 1)

	cases := []struct {
		name, host, origin string
		want               int
	}{
		{"localhost", "localhost:" + p, "", http.StatusOK},
		{"ipv4 loopback", "127.0.0.1:" + p, "", http.StatusOK},
		{"ipv6 loopback", "[::1]:" + p, "", http.StatusOK},
		{"uppercase localhost", "LOCALHOST:" + p, "", http.StatusOK},
		{"same-origin fetch", "127.0.0.1:" + p, "http://127.0.0.1:" + p, http.StatusOK},
		{"rebinding hostname", "evil.example.com:" + p, "", http.StatusForbidden},
		{"rebinding without port", "evil.example.com", "", http.StatusForbidden},
		{"lookalike host", "localhost.evil.example.com:" + p, "", http.StatusForbidden},
		{"other port", "localhost:" + other, "", http.StatusForbidden},
		{"no port (80)", "localhost", "", http.StatusForbidden},
		{"remote origin", "localhost:" + p, "https://evil.example.com", http.StatusForbidden},
		{"local origin other port", "localhost:" + p, "http://localhost:" + other, http.StatusForbidden},
		{"https origin", "localhost:" + p, "https://localhost:" + p, http.StatusForbidden},
		{"null origin", "localhost:" + p, "null", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serve(s, tc.host, tc.origin).Code; got != tc.want {
				t.Fatalf("Host %q Origin %q: got %d, want %d", tc.host, tc.origin, got, tc.want)
			}
		})
	}
}

// TestDoctorOverHTTP_HidesGitIdentity checks that the git name and email shown
// by `agyo doctor` never reach /api/doctor or /api/all.
func TestDoctorOverHTTP_HidesGitIdentity(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	cfg := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(cfg, []byte("[user]\n\tname = Probe Person\n\temail = probe-person@example.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	s := newTestServer(t)
	for _, path := range []string{"/api/doctor", "/api/all"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = "127.0.0.1:" + strconv.Itoa(s.port)
		w := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		body := w.Body.String()
		for _, secret := range []string{"probe-person@example.invalid", "Probe Person"} {
			if strings.Contains(body, secret) {
				t.Errorf("%s leaks %q", path, secret)
			}
		}
		if !strings.Contains(body, "identity configured") {
			t.Errorf("%s: expected the redacted git check, got %s", path, body)
		}
	}
}
