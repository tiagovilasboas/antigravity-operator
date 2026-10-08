package profile_test

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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagoboas/antigravity-operator/internal/profile"
)

func TestCDP_ListTabs_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json" {
			tabs := []profile.Tab{
				{
					ID:    "tab1",
					Title: "Google AI",
					Type:  "page",
					URL:   "https://discuss.ai.google.dev",
				},
				{
					ID:    "sw1",
					Title: "Service Worker",
					Type:  "service_worker",
					URL:   "https://discuss.ai.google.dev/sw.js",
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

	tabs, err := profile.ListTabs(port)
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}

	if len(tabs) != 1 {
		t.Fatalf("expected 1 page tab (filtering out service_worker), got %d", len(tabs))
	}
	if tabs[0].Title != "Google AI" {
		t.Errorf("expected title 'Google AI', got '%s'", tabs[0].Title)
	}
}

func TestCDP_OpenAndCloseTab_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/json/new") {
			_ = json.NewEncoder(w).Encode(profile.Tab{
				ID:    "new-tab-id",
				Title: "New Tab",
				Type:  "page",
				URL:   "https://github.com",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/json/close/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Target is closing"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	parts := strings.Split(server.URL, ":")
	port, _ := strconv.Atoi(parts[len(parts)-1])

	tab, err := profile.OpenTab(port, "https://github.com")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	if tab.ID != "new-tab-id" {
		t.Errorf("expected new-tab-id, got %s", tab.ID)
	}

	if err := profile.CloseTab(port, "new-tab-id"); err != nil {
		t.Fatalf("CloseTab failed: %v", err)
	}
}

func TestCDP_Eval_NoTabs(t *testing.T) {
	// Inactive port
	_, err := profile.Eval(59997, "1+1")
	if err == nil {
		t.Error("expected error for inactive port on Eval, got nil")
	}
}

func TestCDP_Screenshot_NoTabs(t *testing.T) {
	// Inactive port
	err := profile.Screenshot(59997, "/tmp/should_fail.png")
	if err == nil {
		t.Error("expected error for inactive port on Screenshot, got nil")
	}
}

// TestCDP_EvalAndScreenshot_WithMockWS testa o fluxo completo de Eval e Screenshot
// usando um listener TCP que simula o handshake e frames RFC 6455 do Chrome DevTools.
func TestCDP_EvalAndScreenshot_WithMockWS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Goroutine que atende tanto HTTP (/json) quanto o WebSocket upgrade
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockCDPConn(conn, port)
		}
	}()

	// 1. Testa Eval
	val, err := profile.Eval(port, "document.title")
	if err != nil {
		t.Fatalf("Eval with mock failed: %v", err)
	}
	if !strings.Contains(val, "Test Page") {
		t.Errorf("expected val to contain 'Test Page', got %s", val)
	}

	// 2. Testa Screenshot
	tmpImg := filepath.Join(t.TempDir(), "shot.png")
	err = profile.Screenshot(port, tmpImg)
	if err != nil {
		t.Fatalf("Screenshot with mock failed: %v", err)
	}
	if fi, err := os.Stat(tmpImg); err != nil || fi.Size() == 0 {
		t.Errorf("expected screenshot file to be written, err: %v", err)
	}
}

func handleMockCDPConn(conn net.Conn, port int) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	reqLine, err := r.ReadString('\n')
	if err != nil {
		return
	}

	// Se for requisição HTTP GET /json
	if strings.HasPrefix(reqLine, "GET /json") && !strings.Contains(reqLine, "devtools") {
		// Drena headers
		for {
			line, err := r.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
		}
		tabs := []profile.Tab{
			{
				ID:                   "test-tab-1",
				Title:                "Test Page",
				Type:                 "page",
				URL:                  "http://example.com",
				WebSocketDebuggerURL: fmt.Sprintf("ws://127.0.0.1:%d/devtools/page/test-tab-1", port),
			},
		}
		body, _ := json.Marshal(tabs)
		resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), string(body))
		_, _ = conn.Write([]byte(resp))
		return
	}

	// Se for WebSocket upgrade
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

		// Calcula Sec-WebSocket-Accept
		h := sha1.New()
		h.Write([]byte(secKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

		upgradeResp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptKey)
		_, _ = conn.Write([]byte(upgradeResp))

		// Lê mensagem do cliente (Eval ou Screenshot)
		_, err = r.ReadByte() // b0
		if err != nil {
			return
		}
		b1, err := r.ReadByte()
		if err != nil {
			return
		}
		length := int(b1 & 0x7F)
		mask := make([]byte, 4)
		_, _ = io.ReadFull(r, mask)
		payload := make([]byte, length)
		_, _ = io.ReadFull(r, payload)
		for i := 0; i < length; i++ {
			payload[i] ^= mask[i%4]
		}

		var reqData map[string]interface{}
		_ = json.Unmarshal(payload, &reqData)
		method, _ := reqData["method"].(string)

		var respPayload []byte
		if method == "Runtime.evaluate" {
			respPayload = []byte(`{"id":1,"result":{"result":{"type":"string","value":"Test Page"}}}`)
		} else if method == "Page.captureScreenshot" {
			// PNG simples em base64 (1x1 transparente)
			tinyPNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
			respPayload = []byte(fmt.Sprintf(`{"id":1,"result":{"data":"%s"}}`, tinyPNG))
		}

		// Envia frame WS sem máscara (servidor -> cliente)
		var out bytes.Buffer
		out.WriteByte(0x81) // FIN + text
		out.WriteByte(byte(len(respPayload)))
		out.Write(respPayload)
		_, _ = conn.Write(out.Bytes())
	}
}

func TestCDP_FindTabAndEnsureTab(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json" {
			tabs := []profile.Tab{
				{
					ID:    "tab-nlm",
					Title: "NotebookLM",
					Type:  "page",
					URL:   "https://notebooklm.google.com/notebook/123",
				},
				{
					ID:    "tab-other",
					Title: "Google",
					Type:  "page",
					URL:   "https://google.com",
				},
			}
			_ = json.NewEncoder(w).Encode(tabs)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/json/new") {
			_ = json.NewEncoder(w).Encode(profile.Tab{
				ID:    "tab-created",
				Title: "New Target",
				Type:  "page",
				URL:   "https://notebooklm.google.com",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	parts := strings.Split(server.URL, ":")
	port, _ := strconv.Atoi(parts[len(parts)-1])

	// 1. Find existing tab
	tab, err := profile.FindTab(port, "notebooklm.google.com")
	if err != nil {
		t.Fatalf("FindTab failed: %v", err)
	}
	if tab == nil || tab.ID != "tab-nlm" {
		t.Fatalf("expected tab-nlm, got %+v", tab)
	}

	// 2. Find nonexistent tab
	notFound, err := profile.FindTab(port, "not-found.domain.com")
	if err != nil {
		t.Fatalf("FindTab failed on not found: %v", err)
	}
	if notFound != nil {
		t.Fatalf("expected nil tab for not-found, got %+v", notFound)
	}

	// 3. EnsureTab when exists
	ensured, err := profile.EnsureTab(port, "https://notebooklm.google.com", "notebooklm.google.com")
	if err != nil {
		t.Fatalf("EnsureTab failed: %v", err)
	}
	if ensured.ID != "tab-nlm" {
		t.Fatalf("expected existing tab-nlm, got %s", ensured.ID)
	}

	// 4. EnsureTab when does not exist
	ensuredNew, err := profile.EnsureTab(port, "https://notebooklm.google.com", "missing-url")
	if err != nil {
		t.Fatalf("EnsureTab failed on new: %v", err)
	}
	if ensuredNew.ID != "tab-created" {
		t.Fatalf("expected newly opened tab-created, got %s", ensuredNew.ID)
	}
}

