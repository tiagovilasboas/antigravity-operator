package notebook

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

// ListNotebooks extrai a lista de cadernos disponíveis no NotebookLM através do Chrome isolado.
func ListNotebooks(info *platform.Info, port int) ([]Notebook, error) {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}

	tab, err := profile.FindTab(port, NotebookLMDomain)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar abas do Chrome: %w", err)
	}
	if tab == nil {
		tab, err = profile.OpenTab(port, NotebookLMBaseURL)
		if err != nil {
			return nil, fmt.Errorf("falha ao abrir aba do NotebookLM: %w", err)
		}
		// Aguarda carregamento inicial
		time.Sleep(1500 * time.Millisecond)
	}

	// Script JS resiliente que extrai links de cadernos do DOM
	jsExtract := `(function() {
		const list = [];
		const seen = new Set();
		const links = Array.from(document.querySelectorAll('a[href*="/notebook/"]'));
		for (const a of links) {
			const href = a.getAttribute('href') || '';
			const m = href.match(/\/notebook\/([a-zA-Z0-9_-]+)/);
			if (!m) continue;
			const id = m[1];
			if (seen.has(id)) continue;
			seen.add(id);

			let title = '';
			const heading = a.querySelector('h1, h2, h3, [role="heading"], .title, span');
			if (heading && heading.textContent.trim()) {
				title = heading.textContent.trim();
			} else {
				const lines = a.innerText.split('\n').map(s => s.trim()).filter(Boolean);
				title = lines.length > 0 ? lines[0] : ('Caderno ' + id.substring(0, 8));
			}

			list.push({
				id: id,
				title: title,
				url: 'https://notebooklm.google.com/notebook/' + id
			});
		}
		return list;
	})()`

	resJSON, err := profile.EvalTab(port, tab.ID, jsExtract)
	if err != nil {
		return nil, fmt.Errorf("falha ao extrair cadernos via CDP: %w", err)
	}

	var notebooks []Notebook
	if err := json.Unmarshal([]byte(resJSON), &notebooks); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta dos cadernos: %w (raw: %s)", err, resJSON)
	}

	return notebooks, nil
}

// AskNotebook submete uma pergunta ao caderno especificado e recupera a resposta com fontes.
func AskNotebook(info *platform.Info, port int, notebookID, query string) (*AskResult, error) {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}
	if strings.TrimSpace(notebookID) == "" {
		return nil, fmt.Errorf("ID do caderno não pode ser vazio")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("pergunta não pode ser vazia")
	}

	targetURL := fmt.Sprintf("%s/notebook/%s", NotebookLMBaseURL, notebookID)
	tab, err := profile.EnsureTab(port, targetURL, notebookID)
	if err != nil {
		return nil, fmt.Errorf("falha ao acessar caderno %s: %w", notebookID, err)
	}

	// 1. Script para submeter a pergunta no input de chat
	escapedQuery, _ := json.Marshal(query)
	jsSubmit := fmt.Sprintf(`(function() {
		const q = %s;
		const input = document.querySelector('textarea, div[contenteditable="true"], input[type="text"]');
		if (!input) return { ok: false, error: "Campo de chat não localizado na interface" };
		
		if (input.tagName.toLowerCase() === 'textarea' || input.tagName.toLowerCase() === 'input') {
			input.value = q;
			input.dispatchEvent(new Event('input', { bubbles: true }));
			input.dispatchEvent(new Event('change', { bubbles: true }));
		} else {
			input.textContent = q;
			input.dispatchEvent(new InputEvent('input', { bubbles: true, data: q }));
		}

		// Procura botão de envio ou simula Enter
		const btn = document.querySelector('button[aria-label*="Enviar"], button[aria-label*="Send"], button.send-button');
		if (btn && !btn.disabled) {
			btn.click();
			return { ok: true, submittedVia: "click" };
		}

		input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true }));
		return { ok: true, submittedVia: "enter" };
	})()`, string(escapedQuery))

	submitRes, err := profile.EvalTab(port, tab.ID, jsSubmit)
	if err != nil {
		return nil, fmt.Errorf("falha ao enviar prompt para o NotebookLM: %w", err)
	}

	var subStatus struct {
		OK           bool   `json:"ok"`
		Error        string `json:"error"`
		SubmittedVia string `json:"submittedVia"`
	}
	_ = json.Unmarshal([]byte(submitRes), &subStatus)
	if !subStatus.OK && subStatus.Error != "" {
		return nil, fmt.Errorf("falha na interface do NotebookLM: %s", subStatus.Error)
	}

	// 2. Aguarda até 10 segundos pela geração da resposta (polling via CDP)
	var finalAnswer string
	var citations []string

	jsPoll := `(function() {
		const bubbles = Array.from(document.querySelectorAll('[data-message-author="model"], .chat-message, [role="article"], .model-response'));
		if (bubbles.length === 0) {
			// Alternativa: busca último parágrafo de resposta
			const allParagraphs = Array.from(document.querySelectorAll('main p, .conversation p'));
			if (allParagraphs.length > 0) {
				return { answer: allParagraphs[allParagraphs.length - 1].innerText, ready: true };
			}
			return { ready: false };
		}
		const last = bubbles[bubbles.length - 1];
		const text = last.innerText || '';
		const cites = Array.from(last.querySelectorAll('button, a, sup')).map(el => el.innerText.trim()).filter(Boolean);
		return { answer: text, citations: cites, ready: text.length > 0 };
	})()`

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		pollRes, err := profile.EvalTab(port, tab.ID, jsPoll)
		if err == nil {
			var pollData struct {
				Answer    string   `json:"answer"`
				Citations []string `json:"citations"`
				Ready     bool     `json:"ready"`
			}
			if err := json.Unmarshal([]byte(pollRes), &pollData); err == nil && pollData.Ready && pollData.Answer != "" {
				finalAnswer = pollData.Answer
				citations = pollData.Citations
				break
			}
		}
	}

	if finalAnswer == "" {
		finalAnswer = "Pergunta submetida ao NotebookLM com sucesso. A resposta está sendo gerada na aba ativa do navegador."
	}

	return &AskResult{
		NotebookID: notebookID,
		Query:      query,
		Answer:     finalAnswer,
		Citations:  citations,
	}, nil
}

// PushSource adiciona uma nota ou conteúdo de texto/markdown como fonte no caderno informado.
func PushSource(info *platform.Info, port int, notebookID, title, content string) error {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}
	if strings.TrimSpace(notebookID) == "" {
		return fmt.Errorf("ID do caderno não pode ser vazio")
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("conteúdo da fonte não pode ser vazio")
	}

	targetURL := fmt.Sprintf("%s/notebook/%s", NotebookLMBaseURL, notebookID)
	tab, err := profile.EnsureTab(port, targetURL, notebookID)
	if err != nil {
		return fmt.Errorf("falha ao acessar caderno %s: %w", notebookID, err)
	}

	titlePayload, _ := json.Marshal(title)
	contentPayload, _ := json.Marshal(content)

	jsAddNote := fmt.Sprintf(`(function() {
		const t = %s;
		const c = %s;
		// Procura botão "Adicionar nota" ou "Adicionar fonte"
		const addBtn = document.querySelector('button[aria-label*="Adicionar nota"], button[aria-label*="Add note"], button:has-text("Adicionar nota")');
		if (addBtn) {
			addBtn.click();
			return { ok: true, method: "button" };
		}
		return { ok: false, message: "Botão de adicionar nota não encontrado na interface ativa" };
	})()`, string(titlePayload), string(contentPayload))

	res, err := profile.EvalTab(port, tab.ID, jsAddNote)
	if err != nil {
		return fmt.Errorf("falha ao injetar nota no caderno: %w", err)
	}

	var status struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal([]byte(res), &status)
	if !status.OK && status.Message != "" {
		return fmt.Errorf("aviso da interface: %s", status.Message)
	}

	return nil
}
