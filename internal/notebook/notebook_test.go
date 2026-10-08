package notebook_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagoboas/antigravity-operator/internal/notebook"
	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/internal/profile"
)

func TestNotebook_CheckSession_Inactive(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	st := notebook.CheckSession(info, 59998)
	if st.ChromeRunning {
		t.Errorf("expected ChromeRunning=false for dead port, got true")
	}
	if !strings.Contains(st.Message, "inativo") {
		t.Errorf("expected message to mention 'inativo', got: %s", st.Message)
	}
}

func TestNotebook_CheckSession_ActiveTabs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			_ = json.NewEncoder(w).Encode(profile.ChromeVersionResponse{
				Browser: "Chrome/128.0",
			})
			return
		}
		if r.URL.Path == "/json" {
			tabs := []profile.Tab{
				{
					ID:    "tab1",
					Title: "NotebookLM",
					Type:  "page",
					URL:   "https://notebooklm.google.com/notebook/abc-123",
				},
			}
			_ = json.NewEncoder(w).Encode(tabs)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	parts := strings.Split(server.URL, ":")
	port, _ := strconv.Atoi(parts[len(parts)-1])

	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	st := notebook.CheckSession(info, port)
	if !st.ChromeRunning {
		t.Errorf("expected ChromeRunning=true")
	}
	if !st.HasTab {
		t.Errorf("expected HasTab=true")
	}
	if !st.IsLoggedIn {
		t.Errorf("expected IsLoggedIn=true")
	}
	if st.TabID != "tab1" {
		t.Errorf("expected TabID=tab1, got %s", st.TabID)
	}
}

func TestNotebook_CheckSession_LoginRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			_ = json.NewEncoder(w).Encode(profile.ChromeVersionResponse{
				Browser: "Chrome/128.0",
			})
			return
		}
		if r.URL.Path == "/json" {
			tabs := []profile.Tab{
				{
					ID:    "tab-login",
					Title: "Sign in - Google Accounts",
					Type:  "page",
					URL:   "https://accounts.google.com/v3/signin/identifier?continue=https://notebooklm.google.com",
				},
			}
			_ = json.NewEncoder(w).Encode(tabs)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	parts := strings.Split(server.URL, ":")
	port, _ := strconv.Atoi(parts[len(parts)-1])

	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	st := notebook.CheckSession(info, port)
	if !st.ChromeRunning {
		t.Errorf("expected ChromeRunning=true")
	}
	if st.IsLoggedIn {
		t.Errorf("expected IsLoggedIn=false when in accounts.google.com")
	}
	if !strings.Contains(st.Message, "aguardando autenticação") {
		t.Errorf("expected message about awaiting authentication, got: %s", st.Message)
	}
}

func TestNotebook_ServeMCP_Lifecycle(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	// Simula fluxo de mensagens do cliente MCP para o agyo
	var in bytes.Buffer
	// 1. initialize
	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	// 2. ping
	in.WriteString(`{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n")
	// 3. tools/list
	in.WriteString(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}` + "\n")
	// 4. tools/call notebooklm_status
	in.WriteString(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"notebooklm_status","arguments":{}}}` + "\n")
	// 5. unknown method
	in.WriteString(`{"jsonrpc":"2.0","id":5,"method":"unknown_test_method"}` + "\n")

	var out bytes.Buffer
	err := notebook.ServeMCP(info, &in, &out)
	if err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 JSON-RPC response lines, got %d: %v", len(lines), lines)
	}

	// Verifica resposta de initialize
	var initResp notebook.JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("failed to parse init resp: %v", err)
	}
	if initResp.Error != nil {
		t.Errorf("unexpected error in initResp: %v", initResp.Error)
	}

	// Verifica resposta de ping
	var pingResp notebook.JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &pingResp); err != nil {
		t.Fatalf("failed to parse ping resp: %v", err)
	}
	if pingResp.Error != nil {
		t.Errorf("unexpected error in pingResp: %v", pingResp.Error)
	}

	// Verifica tools/list
	var toolsResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &toolsResp); err != nil {
		t.Fatalf("failed to parse toolsResp: %v", err)
	}
	if len(toolsResp.Result.Tools) < 4 {
		t.Errorf("expected at least 4 tools, got %d", len(toolsResp.Result.Tools))
	}

	// Verifica tools/call notebooklm_status
	var callResp struct {
		JSONRPC string                  `json:"jsonrpc"`
		ID      int                     `json:"id"`
		Result  notebook.ToolCallResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[3]), &callResp); err != nil {
		t.Fatalf("failed to parse callResp: %v", err)
	}
	if len(callResp.Result.Content) == 0 {
		t.Errorf("expected content in callResp result")
	}

	// Verifica unknown method error
	var errResp notebook.JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[4]), &errResp); err != nil {
		t.Fatalf("failed to parse errResp: %v", err)
	}
	if errResp.Error == nil || errResp.Error.Code != -32601 {
		t.Errorf("expected error code -32601 for unknown method, got: %+v", errResp.Error)
	}
}
