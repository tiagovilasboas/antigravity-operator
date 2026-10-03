package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagoboas/antigravity-operator/internal/platform"
)

// TestEventsDoNotLeakSessionText garante que /api/events e /api/all expõem só
// metadados (tipo, passo, ferramenta, status) e nunca texto do transcript.
func TestEventsDoNotLeakSessionText(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logs := filepath.Join(home, ".gemini", "brain", "conv-1", ".system_generated", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := strings.Join([]string{
		`{"step_index":1,"type":"USER_INPUT","content":"SECRET_PROMPT deploy with ghp_abcdefghijklmnopqrstuvwxyz0123456789"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","thinking":"SECRET_REASONING about the user","tool_calls":[{"name":"run_command","args":{"CommandLine":"curl -H 'Authorization: SECRET_COMMAND'"}}]}`,
		`{"step_index":3,"type":"GENERIC","status":"DONE","content":"SECRET_OUTPUT of the tool"}`,
		`{"step_index":4,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"invoke_subagent","args":{"Role":"r","Prompt":"SECRET_SUBAGENT task"}},{"name":"send_message","args":{"Recipient":"x","Message":"SECRET_MESSAGE body"}}]}`,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(logs, "transcript.jsonl"), []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}

	// Sem NewServer: os handlers só precisam da config e não abrimos porta.
	server := &Server{cfg: Config{
		TargetDir:    t.TempDir(),
		PlatformInfo: &platform.Info{OS: "linux", Arch: "amd64"},
	}}

	for _, tc := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/api/events", server.handleEvents},
		{"/api/all", server.handleAll},
	} {
		w := httptest.NewRecorder()
		tc.handler(w, httptest.NewRequest("GET", tc.path, nil))
		body := w.Body.String()
		if w.Code != http.StatusOK {
			t.Fatalf("%s: esperava 200, obteve %d", tc.path, w.Code)
		}
		for _, leak := range []string{"SECRET_", "ghp_", "Authorization", "curl"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s vaza texto da sessão (%q): %s", tc.path, leak, body)
			}
		}
		for _, want := range []string{"[User #1]", "[Think #2]", "run_command", "[Step #3] Status: DONE", "invoke_subagent", "send_message"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s deveria conter metadado %q: %s", tc.path, want, body)
			}
		}
	}
}
