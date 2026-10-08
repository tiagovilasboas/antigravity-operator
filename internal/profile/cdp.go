package profile

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Tab representa uma aba ou alvo de inspeção do Chrome DevTools.
type Tab struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// ListTabs retorna todas as abas e alvos do tipo "page" abertos no Chrome.
func ListTabs(port int) ([]Tab, error) {
	if port <= 0 {
		port = DefaultDebugPort
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json", port)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar no Chrome na porta %d: %w", port, err)
	}
	defer resp.Body.Close()

	var all []Tab
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("falha ao decodificar abas do Chrome: %w", err)
	}

	var pages []Tab
	for _, t := range all {
		if t.Type == "page" {
			pages = append(pages, t)
		}
	}
	return pages, nil
}

// OpenTab abre uma nova aba com a URL especificada no Chrome gerenciado.
func OpenTab(port int, targetURL string) (*Tab, error) {
	if port <= 0 {
		port = DefaultDebugPort
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/new?%s", port, url.QueryEscape(targetURL))
	client := http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição de nova aba: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir nova aba no Chrome: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		getResp, getErr := client.Get(endpoint)
		if getErr != nil {
			return nil, fmt.Errorf("falha ao abrir nova aba no Chrome: %w", getErr)
		}
		defer getResp.Body.Close()
		resp = getResp
	}

	var tab Tab
	if err := json.NewDecoder(resp.Body).Decode(&tab); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta de nova aba: %w", err)
	}
	return &tab, nil
}

// CloseTab fecha a aba com o ID informado.
func CloseTab(port int, targetID string) error {
	if port <= 0 {
		port = DefaultDebugPort
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", port, targetID)
	client := http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return fmt.Errorf("falha ao criar requisição para fechar aba %s: %w", targetID, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao fechar aba %s: %w", targetID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed {
		getResp, getErr := client.Get(endpoint)
		if getErr == nil {
			_ = getResp.Body.Close()
		}
	}
	return nil
}

// FindTab localiza uma aba ativa cuja URL contenha a substring informada.
func FindTab(port int, urlSubstr string) (*Tab, error) {
	tabs, err := ListTabs(port)
	if err != nil {
		return nil, err
	}
	for i := range tabs {
		if strings.Contains(tabs[i].URL, urlSubstr) {
			return &tabs[i], nil
		}
	}
	return nil, nil
}

// EnsureTab retorna a aba existente que casa com urlSubstr ou abre uma nova aba com targetURL.
func EnsureTab(port int, targetURL string, urlSubstr string) (*Tab, error) {
	tab, err := FindTab(port, urlSubstr)
	if err != nil {
		return nil, err
	}
	if tab != nil {
		return tab, nil
	}
	return OpenTab(port, targetURL)
}

// EvalTab executa uma expressão JavaScript em uma aba específica identificada pelo ID.
func EvalTab(port int, tabID string, expression string) (string, error) {
	tabs, err := ListTabs(port)
	if err != nil {
		return "", err
	}
	for _, t := range tabs {
		if t.ID == tabID || strings.HasPrefix(t.ID, tabID) {
			if t.WebSocketDebuggerURL == "" {
				return "", fmt.Errorf("a aba %s não possui endpoint de depuração WebSocket", tabID)
			}
			return evalWebSocket(t.WebSocketDebuggerURL, expression)
		}
	}
	return "", fmt.Errorf("aba com ID '%s' não encontrada", tabID)
}

// Eval executa uma expressão JavaScript na primeira aba ativa e retorna o valor serializado.
func Eval(port int, expression string) (string, error) {
	tabs, err := ListTabs(port)
	if err != nil {
		return "", err
	}
	if len(tabs) == 0 {
		return "", fmt.Errorf("nenhuma aba ativa encontrada no Chrome")
	}

	wsURL := tabs[0].WebSocketDebuggerURL
	if wsURL == "" {
		return "", fmt.Errorf("a aba ativa não possui endpoint de depuração WebSocket")
	}

	return evalWebSocket(wsURL, expression)
}

func evalWebSocket(wsURL string, expression string) (string, error) {
	client, err := dialCDP(wsURL)
	if err != nil {
		return "", fmt.Errorf("falha ao conectar no WebSocket do CDP: %w", err)
	}
	defer client.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    expression,
			"returnByValue": true,
		},
	})

	if err := client.Send(payload); err != nil {
		return "", fmt.Errorf("falha ao enviar comando CDP: %w", err)
	}

	respBytes, err := client.Read()
	if err != nil {
		return "", fmt.Errorf("falha ao ler resposta do CDP: %w", err)
	}

	var parsed struct {
		Result struct {
			Result struct {
				Type  string      `json:"type"`
				Value interface{} `json:"value"`
			} `json:"result"`
			ExceptionDetails *struct {
				Text string `json:"text"`
			} `json:"exceptionDetails"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return string(respBytes), nil
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("erro CDP: %s", parsed.Error.Message)
	}
	if parsed.Result.ExceptionDetails != nil {
		return "", fmt.Errorf("exceção JS: %s", parsed.Result.ExceptionDetails.Text)
	}

	valBytes, _ := json.Marshal(parsed.Result.Result.Value)
	return string(valBytes), nil
}

// Screenshot captura a tela da aba ativa e salva o PNG no destino especificado.
func Screenshot(port int, destPath string) error {
	tabs, err := ListTabs(port)
	if err != nil {
		return err
	}
	if len(tabs) == 0 {
		return fmt.Errorf("nenhuma aba ativa encontrada no Chrome")
	}

	wsURL := tabs[0].WebSocketDebuggerURL
	if wsURL == "" {
		return fmt.Errorf("a aba ativa não possui endpoint de depuração WebSocket")
	}

	client, err := dialCDP(wsURL)
	if err != nil {
		return fmt.Errorf("falha ao conectar no WebSocket do CDP: %w", err)
	}
	defer client.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"id":     1,
		"method": "Page.captureScreenshot",
		"params": map[string]interface{}{
			"format": "png",
		},
	})

	if err := client.Send(payload); err != nil {
		return fmt.Errorf("falha ao disparar captura de tela: %w", err)
	}

	respBytes, err := client.Read()
	if err != nil {
		return fmt.Errorf("falha ao receber imagem do CDP: %w", err)
	}

	var parsed struct {
		Result struct {
			Data string `json:"data"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return fmt.Errorf("falha ao decodificar resposta da captura: %w", err)
	}

	if parsed.Result.Data == "" {
		return fmt.Errorf("dados de imagem vazios retornados pelo Chrome")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(parsed.Result.Data)
	if err != nil {
		return fmt.Errorf("falha ao decodificar base64 da imagem: %w", err)
	}

	return os.WriteFile(destPath, imgBytes, 0644)
}

// cdpWSConn implementa um cliente WebSocket minimalista (RFC 6455) usando apenas a biblioteca padrão do Go.
type cdpWSConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialCDP(wsURL string) (*cdpWSConn, error) {
	wsURL = strings.TrimPrefix(wsURL, "ws://")
	parts := strings.SplitN(wsURL, "/", 2)
	host := parts[0]
	path := "/"
	if len(parts) > 1 {
		path = "/" + parts[1]
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, err
	}

	key := make([]byte, 16)
	_, _ = rand.Read(key)
	secKey := base64.StdEncoding.EncodeToString(key)

	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, host, secKey)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}

	return &cdpWSConn{conn: conn, r: r}, nil
}

func (c *cdpWSConn) Send(payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x81) // FIN + Opcode 1 (Text)

	length := len(payload)
	maskKey := make([]byte, 4)
	_, _ = rand.Read(maskKey)

	if length <= 125 {
		buf.WriteByte(byte(length) | 0x80)
	} else if length <= 65535 {
		buf.WriteByte(126 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint16(length))
	} else {
		buf.WriteByte(127 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint64(length))
	}

	buf.Write(maskKey)

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}
	buf.Write(masked)

	_, err := c.conn.Write(buf.Bytes())
	return err
}

func (c *cdpWSConn) Read() ([]byte, error) {
	_, err := c.r.ReadByte()
	if err != nil {
		return nil, err
	}
	b1, err := c.r.ReadByte()
	if err != nil {
		return nil, err
	}

	length := int(b1 & 0x7F)
	if length == 126 {
		var l uint16
		if err := binary.Read(c.r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		length = int(l)
	} else if length == 127 {
		var l uint64
		if err := binary.Read(c.r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		length = int(l)
	}

	isMasked := (b1 & 0x80) != 0
	var maskKey []byte
	if isMasked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(c.r, maskKey); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return nil, err
	}

	if isMasked {
		for i := 0; i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, nil
}

func (c *cdpWSConn) Close() error {
	return c.conn.Close()
}
