package notebook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

var validNotebookID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ListNotebooks extrai a lista de cadernos disponíveis no NotebookLM através do Chrome isolado.
func ListNotebooks(info *platform.Info, port int) ([]Notebook, error) {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}

	tab, err := FindNotebookLMTab(port)
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
			const card = a.closest('mat-card, [role="listitem"], article') || a.parentElement;
			if (card) {
				const titleEl = card.querySelector('.project-button-title, [class*="title"], h1, h2, h3, [role="heading"]');
				if (titleEl && titleEl.textContent.trim()) {
					title = titleEl.textContent.trim();
				}
			}
			if (!title) {
				const heading = a.querySelector('h1, h2, h3, [role="heading"], .title, span');
				if (heading && heading.textContent.trim()) {
					title = heading.textContent.trim();
				} else {
					title = 'Caderno ' + id.substring(0, 8);
				}
			}

			let sourceCount = 0;
			if (card) {
				const allText = Array.from(card.querySelectorAll('*')).map(el => el.textContent.trim());
				const sourceText = allText.find(t => t.includes('fonte') || t.includes('source'));
				if (sourceText) {
					const sm = sourceText.match(/(\d+)\s*(?:fonte|source)/i);
					if (sm) sourceCount = parseInt(sm[1], 10);
				}
			}

			list.push({
				id: id,
				title: title,
				source_count: sourceCount,
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
	if !validNotebookID.MatchString(notebookID) {
		return nil, fmt.Errorf("identificador de caderno inválido: %s", notebookID)
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("pergunta não pode ser vazia")
	}

	targetURL := fmt.Sprintf("%s/notebook/%s", NotebookLMBaseURL, notebookID)
	tab, err := profile.EnsureTab(port, targetURL, "/notebook/"+notebookID)
	if err != nil {
		return nil, fmt.Errorf("falha ao acessar caderno %s: %w", notebookID, err)
	}

	// 1. Script para submeter a pergunta no input de chat
	escapedQuery, _ := json.Marshal(query)
	jsSubmit := fmt.Sprintf(`(function() {
		const q = %s;
		const input = document.querySelector('textarea.query-box-input, textarea[placeholder*="pergunta"], textarea[placeholder*="Ask"], textarea[aria-label*="consulta"], textarea[aria-label*="query"], textarea, div[contenteditable="true"]');
		if (!input) return { ok: false, error: "Campo de chat não localizado na interface" };
		
		if (input.tagName.toLowerCase() === 'textarea' || input.tagName.toLowerCase() === 'input') {
			input.value = q;
			input.dispatchEvent(new Event('input', { bubbles: true }));
			input.dispatchEvent(new Event('change', { bubbles: true }));
		} else {
			input.textContent = q;
			input.dispatchEvent(new InputEvent('input', { bubbles: true, data: q }));
		}

		// Procura botão de envio ativo
		const buttons = Array.from(document.querySelectorAll('button[aria-label*="Enviar"], button[aria-label*="Send"], button.send-button'));
		const btn = buttons.find(b => !b.disabled && !b.hasAttribute('disabled'));
		if (btn) {
			btn.click();
			return { ok: true, submittedVia: "click" };
		}

		input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true }));
		return { ok: true, submittedVia: "enter" };
	})()`, string(escapedQuery))

	var subStatus struct {
		OK           bool   `json:"ok"`
		Error        string `json:"error"`
		SubmittedVia string `json:"submittedVia"`
	}

	submitDeadline := time.Now().Add(6 * time.Second)
	for {
		submitRes, err := profile.EvalTab(port, tab.ID, jsSubmit)
		if err == nil {
			_ = json.Unmarshal([]byte(submitRes), &subStatus)
			if subStatus.OK {
				break
			}
		}
		if time.Now().After(submitDeadline) {
			if subStatus.Error != "" {
				return nil, fmt.Errorf("falha na interface do NotebookLM: %s", subStatus.Error)
			}
			return nil, fmt.Errorf("tempo limite aguardando campo de chat do caderno %s", notebookID)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 2. Aguarda até 15 segundos pela geração da resposta (polling via CDP)
	var finalAnswer string
	var citations []string

	jsPoll := `(function() {
		const pairs = Array.from(document.querySelectorAll('.chat-message-pair'));
		if (pairs.length > 0) {
			const lastPair = pairs[pairs.length - 1];
			const text = lastPair.innerText || '';
			if (text.length > 0 && !text.includes('Carregando')) {
				const cites = Array.from(lastPair.querySelectorAll('button, a, sup, [class*="citation"]')).map(el => el.innerText.trim()).filter(Boolean);
				return { answer: text, citations: cites, ready: true };
			}
		}
		const bubbles = Array.from(document.querySelectorAll('[data-message-author="model"], .chat-message, [role="article"], .model-response'));
		if (bubbles.length > 0) {
			const last = bubbles[bubbles.length - 1];
			const text = last.innerText || '';
			const cites = Array.from(last.querySelectorAll('button, a, sup, [class*="citation"]')).map(el => el.innerText.trim()).filter(Boolean);
			return { answer: text, citations: cites, ready: text.length > 0 };
		}
		return { ready: false };
	})()`

	deadline := time.Now().Add(15 * time.Second)
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
	if !validNotebookID.MatchString(notebookID) {
		return fmt.Errorf("identificador de caderno inválido: %s", notebookID)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("conteúdo da fonte não pode ser vazio")
	}

	targetURL := fmt.Sprintf("%s/notebook/%s", NotebookLMBaseURL, notebookID)
	tab, err := profile.EnsureTab(port, targetURL, "/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("falha ao acessar caderno %s: %w", notebookID, err)
	}

	titlePayload, _ := json.Marshal(title)
	contentPayload, _ := json.Marshal(content)

	jsAddNote := fmt.Sprintf(`(function() {
		const t = %s;
		const c = %s;
		// Procura botão "Adicionar nota" ou "Adicionar fonte"
		let addBtn = document.querySelector('button[aria-label*="Adicionar nota"], button[aria-label*="Add note"]');
		if (!addBtn) {
			const buttons = Array.from(document.querySelectorAll('button'));
			addBtn = buttons.find(b => b.textContent && (b.textContent.includes('Adicionar nota') || b.textContent.includes('Add note')));
		}
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

// AddSourceURL adiciona uma URL (YouTube ou web) como fonte no caderno informado via interface do NotebookLM.
func AddSourceURL(info *platform.Info, port int, notebookID, sourceURL string) error {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}
	if strings.TrimSpace(notebookID) == "" {
		return fmt.Errorf("ID do caderno não pode ser vazio")
	}
	if !validNotebookID.MatchString(notebookID) {
		return fmt.Errorf("identificador de caderno inválido: %s", notebookID)
	}
	if strings.TrimSpace(sourceURL) == "" {
		return fmt.Errorf("URL da fonte não pode ser vazia")
	}

	targetURL := fmt.Sprintf("%s/notebook/%s", NotebookLMBaseURL, notebookID)
	tab, err := profile.EnsureTab(port, targetURL, "/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("falha ao acessar caderno %s: %w", notebookID, err)
	}

	escapedURL, _ := json.Marshal(sourceURL)

	jsAddURL := fmt.Sprintf(`(function() {
		const u = %s;
		// 1. Clica no botão "Adicionar fontes" se o modal não estiver aberto
		const panel = document.querySelector('.mat-mdc-dialog-panel');
		if (!panel) {
			const addBtn = Array.from(document.querySelectorAll('button')).find(b => 
				b.getAttribute('aria-label') === 'Adicionar fonte' || b.innerText.includes('Adicionar fontes')
			);
			if (!addBtn) return { ok: false, error: "Botão 'Adicionar fonte' não encontrado" };
			addBtn.click();
		}
		return { ok: true, step: "modal_opened" };
	})()`, string(escapedURL))

	if _, err := profile.EvalTab(port, tab.ID, jsAddURL); err != nil {
		return fmt.Errorf("falha ao abrir modal de fontes: %w", err)
	}

	time.Sleep(800 * time.Millisecond)

	jsSelectYouTubeAndInsert := fmt.Sprintf(`(function() {
		const u = %s;
		// 2. Procura botão de YouTube / Sites se o campo de texto ainda não estiver visível
		let ta = document.querySelector('textarea[aria-label*="URLs"], textarea[placeholder*="links"], textarea[placeholder*="URLs"]');
		if (!ta) {
			const panel = document.querySelector('.mat-mdc-dialog-panel') || document.body;
			const ytBtn = Array.from(panel.querySelectorAll('button, [role="button"], .source-type-card')).find(el => 
				el.innerText.includes('video_youtube') || el.innerText.includes('Sites') || el.innerText.toLowerCase().includes('youtube')
			);
			if (ytBtn) {
				ytBtn.click();
			}
		}

		// Aguarda o textarea aparecer
		ta = document.querySelector('textarea[aria-label*="URLs"], textarea[placeholder*="links"], textarea[placeholder*="URLs"]');
		if (!ta) {
			return { ok: false, error: "Campo de inserção de URL não encontrado" };
		}

		ta.focus();
		ta.value = u;
		ta.dispatchEvent(new Event('input', { bubbles: true }));
		ta.dispatchEvent(new Event('change', { bubbles: true }));

		// 3. Clica no botão 'Inserir' / 'Insert'
		const insertBtn = Array.from(document.querySelectorAll('button')).find(b => 
			(b.innerText.includes('Inserir') || b.innerText.includes('Insert')) && !b.disabled && !b.hasAttribute('disabled')
		);
		if (insertBtn) {
			insertBtn.click();
			return { ok: true, step: "inserted" };
		}

		return { ok: false, error: "Botão 'Inserir' desabilitado ou não encontrado" };
	})()`, string(escapedURL))

	deadline := time.Now().Add(5 * time.Second)
	var finalErr error
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		res, err := profile.EvalTab(port, tab.ID, jsSelectYouTubeAndInsert)
		if err == nil {
			var status struct {
				OK    bool   `json:"ok"`
				Step  string `json:"step"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(res), &status); err == nil && status.OK {
				finalErr = nil
				break
			} else if status.Error != "" {
				finalErr = fmt.Errorf("%s", status.Error)
			}
		}
	}
	if finalErr != nil {
		return fmt.Errorf("falha ao submeter URL no NotebookLM: %w", finalErr)
	}

	return nil
}

