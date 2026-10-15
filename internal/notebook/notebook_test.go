package notebook_test

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
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

func TestNotebook_InputValidation(t *testing.T) {
	info := &platform.Info{}

	// ID inválido com caracteres perigosos
	if _, err := notebook.AskNotebook(info, 9222, "id; injection", "query"); err == nil {
		t.Errorf("expected error on invalid notebook ID in AskNotebook")
	}

	if err := notebook.PushSource(info, 9222, "id/../../traversal", "title", "content"); err == nil {
		t.Errorf("expected error on invalid notebook ID in PushSource")
	}

	// Campos vazios
	if _, err := notebook.AskNotebook(info, 9222, "", "query"); err == nil {
		t.Errorf("expected error on empty notebook ID")
	}
	if _, err := notebook.AskNotebook(info, 9222, "valid-id", ""); err == nil {
		t.Errorf("expected error on empty query")
	}
	if err := notebook.PushSource(info, 9222, "valid-id", "title", ""); err == nil {
		t.Errorf("expected error on empty content")
	}
}

func TestNotebook_AddSourceURL_Validation(t *testing.T) {
	info := &platform.Info{}

	// Empty ID
	if err := notebook.AddSourceURL(info, 9222, "", "https://youtube.com/watch?v=123"); err == nil {
		t.Errorf("expected error on empty notebook ID")
	}

	// Invalid ID
	if err := notebook.AddSourceURL(info, 9222, "invalid;id", "https://youtube.com/watch?v=123"); err == nil {
		t.Errorf("expected error on invalid notebook ID")
	}

	// Empty URL
	if err := notebook.AddSourceURL(info, 9222, "valid-id", ""); err == nil {
		t.Errorf("expected error on empty source URL")
	}

	// Inactive port
	if err := notebook.AddSourceURL(info, 59996, "valid-id", "https://youtube.com/watch?v=123"); err == nil {
		t.Errorf("expected error on inactive port")
	}
}

func TestNotebook_MCPToolExecutionCoverage(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	callTool := func(toolName string, args map[string]interface{}) string {
		var in bytes.Buffer
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      toolName,
				"arguments": args,
			},
		}
		data, _ := json.Marshal(req)
		in.Write(data)
		in.WriteString("\n")

		var out bytes.Buffer
		_ = notebook.ServeMCP(info, &in, &out)
		return out.String()
	}

	// 1. notebooklm_ask missing args
	out := callTool("notebooklm_ask", map[string]interface{}{"notebook_id": ""})
	if !strings.Contains(out, "obrigatórios") {
		t.Errorf("expected error on missing args for notebooklm_ask: %s", out)
	}

	// 2. notebooklm_push missing args
	out = callTool("notebooklm_push", map[string]interface{}{"notebook_id": "123"})
	if !strings.Contains(out, "obrigatórios") {
		t.Errorf("expected error on missing args for notebooklm_push: %s", out)
	}

	// 3. notebooklm_add_source missing args
	out = callTool("notebooklm_add_source", map[string]interface{}{"notebook_id": "123"})
	if !strings.Contains(out, "obrigatórios") {
		t.Errorf("expected error on missing args for notebooklm_add_source: %s", out)
	}

	// 4. unknown tool
	out = callTool("unknown_tool", map[string]interface{}{})
	if !strings.Contains(out, "não suportada") {
		t.Errorf("expected unknown tool message: %s", out)
	}

	// 5. notebooklm_list with inactive port
	out = callTool("notebooklm_list", map[string]interface{}{})
	if !strings.Contains(out, "Erro ao listar cadernos") {
		t.Errorf("expected error on inactive port for notebooklm_list: %s", out)
	}

	// 6. google_calendar_list_events without auth
	out = callTool("google_calendar_list_events", map[string]interface{}{})
	if !strings.Contains(out, "não autenticado") {
		t.Errorf("expected auth error for google_calendar_list_events: %s", out)
	}

	// 7. google_calendar_create_event missing summary
	out = callTool("google_calendar_create_event", map[string]interface{}{})
	if !strings.Contains(out, "obrigatórios") {
		t.Errorf("expected required fields error for google_calendar_create_event: %s", out)
	}

	// 8. google_docs_read missing document_id
	out = callTool("google_docs_read", map[string]interface{}{})
	if !strings.Contains(out, "obrigatório") {
		t.Errorf("expected required field error for google_docs_read: %s", out)
	}

	// 9. google_docs_create missing title
	out = callTool("google_docs_create", map[string]interface{}{})
	if !strings.Contains(out, "obrigatório") {
		t.Errorf("expected required field error for google_docs_create: %s", out)
	}
}

func TestNotebook_FindTabAndOpenSession_InactivePort(t *testing.T) {
	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	// FindNotebookLMTab on dead port
	_, err := notebook.FindNotebookLMTab(59995)
	if err == nil {
		t.Errorf("expected error for FindNotebookLMTab on dead port")
	}

	// OpenSession on dead port
	_, err = notebook.OpenSession(info, 59995)
	if err == nil {
		t.Errorf("expected error for OpenSession on dead port")
	}
}

func handleMockNotebookCDPConn(conn net.Conn, port int) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	reqLine, err := r.ReadString('\n')
	if err != nil {
		return
	}

	if strings.HasPrefix(reqLine, "GET /json") && !strings.Contains(reqLine, "devtools") {
		for {
			line, err := r.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
		}
		tabs := []profile.Tab{
			{
				ID:                   "tab-nlm-1",
				Title:                "NotebookLM",
				Type:                 "page",
				URL:                  "https://notebooklm.google.com/notebook/test-nb-123",
				WebSocketDebuggerURL: fmt.Sprintf("ws://127.0.0.1:%d/devtools/page/tab-nlm-1", port),
			},
		}
		body, _ := json.Marshal(tabs)
		resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), string(body))
		_, _ = conn.Write([]byte(resp))
		return
	}

	if strings.Contains(reqLine, "devtools") || strings.HasPrefix(reqLine, "GET /devtools") {
		var secKey string
		for {
			line, err := r.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "sec-websocket-key:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					secKey = strings.TrimSpace(parts[1])
				}
			}
		}

		h := sha1.New()
		h.Write([]byte(secKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

		upgradeResp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptKey)
		_, _ = conn.Write([]byte(upgradeResp))

		for {
			_, err = r.ReadByte()
			if err != nil {
				return
			}
			b1, err := r.ReadByte()
			if err != nil {
				return
			}
			var length int
			if b1&0x7F == 126 {
				lenBytes := make([]byte, 2)
				_, err = io.ReadFull(r, lenBytes)
				if err != nil {
					return
				}
				length = int(lenBytes[0])<<8 | int(lenBytes[1])
			} else {
				length = int(b1 & 0x7F)
			}

			mask := make([]byte, 4)
			_, err = io.ReadFull(r, mask)
			if err != nil {
				return
			}
			payload := make([]byte, length)
			_, err = io.ReadFull(r, payload)
			if err != nil {
				return
			}
			for i := 0; i < length; i++ {
				payload[i] ^= mask[i%4]
			}

			var reqData map[string]interface{}
			_ = json.Unmarshal(payload, &reqData)
			reqID, _ := reqData["id"].(float64)
			params, _ := reqData["params"].(map[string]interface{})
			expr, _ := params["expression"].(string)

			var retVal interface{}
			valType := "object"
			if strings.Contains(expr, "links = Array.from") || (strings.Contains(expr, "/notebook/") && strings.Contains(expr, "list.push")) {
				retVal = []map[string]interface{}{
					{"id": "test-nb-123", "title": "Caderno de Redes", "url": "https://notebooklm.google.com/notebook/test-nb-123"},
				}
			} else if strings.Contains(expr, "chat-message-pair") {
				retVal = map[string]interface{}{
					"ready":     true,
					"answer":    "Resposta fundamentada com sucesso.",
					"citations": []string{"https://youtube.com/watch?v=123"},
				}
			} else if strings.Contains(expr, "query-box-input") {
				retVal = map[string]interface{}{
					"ok":           true,
					"submittedVia": "enter",
				}
			} else {
				retVal = map[string]interface{}{"ok": true}
			}

			respObj := map[string]interface{}{
				"id": int(reqID),
				"result": map[string]interface{}{
					"result": map[string]interface{}{
						"type":  valType,
						"value": retVal,
					},
				},
			}
			respPayload, _ := json.Marshal(respObj)

			var frame bytes.Buffer
			frame.WriteByte(0x81)
			if len(respPayload) < 126 {
				frame.WriteByte(byte(len(respPayload)))
			} else {
				frame.WriteByte(126)
				frame.WriteByte(byte(len(respPayload) >> 8))
				frame.WriteByte(byte(len(respPayload) & 0xFF))
			}
			frame.Write(respPayload)
			_, _ = conn.Write(frame.Bytes())
		}
	}
}

func TestNotebook_CDP_Evaluation_FullFlow(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockNotebookCDPConn(conn, port)
		}
	}()

	info := &platform.Info{
		BrowserProfile: t.TempDir(),
	}

	// 1. ListNotebooks
	notebooks, err := notebook.ListNotebooks(info, port)
	if err != nil {
		t.Fatalf("ListNotebooks with mock failed: %v", err)
	}
	if len(notebooks) != 1 || notebooks[0].ID != "test-nb-123" {
		t.Errorf("unexpected notebooks list: %+v", notebooks)
	}

	// 2. AskNotebook
	answer, err := notebook.AskNotebook(info, port, "test-nb-123", "Qual o resumo?")
	if err != nil {
		t.Fatalf("AskNotebook with mock failed: %v", err)
	}
	if !strings.Contains(answer.Answer, "Resposta fundamentada") {
		t.Errorf("unexpected answer: %+v", answer)
	}

	// 3. PushSource
	err = notebook.PushSource(info, port, "test-nb-123", "Título da Nota", "Conteúdo detalhado da nota")
	if err != nil {
		t.Fatalf("PushSource with mock failed: %v", err)
	}

	// 4. AddSourceURL
	err = notebook.AddSourceURL(info, port, "test-nb-123", "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("AddSourceURL with mock failed: %v", err)
	}

	// 5. MCP tools/call on all functions through ServeMCP
	var in bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"notebooklm_list","arguments":{}}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"notebooklm_ask","arguments":{"notebook_id":"test-nb-123","query":"resumo"}}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"notebooklm_push","arguments":{"notebook_id":"test-nb-123","title":"T","content":"C"}}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"notebooklm_add_source","arguments":{"notebook_id":"test-nb-123","url":"https://youtube.com/watch?v=123"}}}` + "\n")

	var out bytes.Buffer
	err = notebook.ServeMCP(info, &in, &out)
	if err != nil {
		t.Fatalf("ServeMCP tools call failed: %v", err)
	}
	outLines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(outLines) != 4 {
		t.Fatalf("expected 4 responses from ServeMCP, got %d: %s", len(outLines), out.String())
	}
}
