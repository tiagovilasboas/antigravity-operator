package profile

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// NavigateTab navega uma aba existente para targetURL via comando CDP Page.navigate.
func NavigateTab(port int, tabID string, targetURL string) (*Tab, error) {
	tabs, err := ListTabs(port)
	if err != nil {
		return nil, err
	}
	for i := range tabs {
		t := &tabs[i]
		if t.ID == tabID || strings.HasPrefix(t.ID, tabID) {
			if t.WebSocketDebuggerURL == "" {
				return nil, fmt.Errorf("a aba %s não possui endpoint WebSocket", tabID)
			}
			client, err := dialCDP(t.WebSocketDebuggerURL)
			if err != nil {
				return nil, fmt.Errorf("falha ao conectar no WebSocket da aba %s: %w", tabID, err)
			}
			defer client.Close()

			payload, _ := json.Marshal(map[string]interface{}{
				"id":     1,
				"method": "Page.navigate",
				"params": map[string]interface{}{
					"url": targetURL,
				},
			})
			if err := client.Send(payload); err != nil {
				return nil, fmt.Errorf("falha ao enviar Page.navigate: %w", err)
			}
			_, _ = client.ReadForID(1)
			t.URL = targetURL
			return t, nil
		}
	}
	return nil, fmt.Errorf("aba com ID '%s' não encontrada", tabID)
}

// OpenOrReuseTab reutiliza uma aba existente que já esteja na mesma URL ou uma aba em branco (about:blank / chrome://newtab/),
// evitando abrir abas duplicadas desnecessárias a menos que forceNewTab seja true.
func OpenOrReuseTab(port int, targetURL string, forceNewTab bool) (*Tab, bool, error) {
	if !forceNewTab {
		if tabs, err := ListTabs(port); err == nil {
			// 1. Verifica se já existe aba com a mesma URL (ou mesma URL base sem query transitória)
			for i := range tabs {
				if strings.EqualFold(strings.TrimRight(tabs[i].URL, "/"), strings.TrimRight(targetURL, "/")) {
					return &tabs[i], true, nil
				}
			}
			// 2. Verifica se existe aba em branco ociosa (about:blank ou chrome://newtab/) para navegar in-place
			for i := range tabs {
				u := strings.ToLower(strings.TrimSpace(tabs[i].URL))
				if u == "about:blank" || strings.HasPrefix(u, "chrome://newtab") || strings.HasPrefix(u, "chrome://startpage") {
					if navTab, navErr := NavigateTab(port, tabs[i].ID, targetURL); navErr == nil {
						return navTab, true, nil
					}
				}
			}
		}
	}

	tab, err := OpenTab(port, targetURL)
	if err != nil {
		return nil, false, err
	}
	return tab, false, nil
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

	respBytes, err := client.ReadForID(1)
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

	respBytes, err := client.ReadForID(1)
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
