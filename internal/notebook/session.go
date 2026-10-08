package notebook

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

const (
	NotebookLMBaseURL = "https://notebooklm.google.com"
	NotebookLMDomain  = "notebooklm.google.com"
)

// FindNotebookLMTab localiza a aba ativa do NotebookLM aceitando os domínios conhecidos.
func FindNotebookLMTab(port int) (*profile.Tab, error) {
	for _, domain := range []string{NotebookLMDomain, "notebook.google.com", "notebooklm.google"} {
		tab, err := profile.FindTab(port, domain)
		if err != nil {
			return nil, err
		}
		if tab != nil {
			return tab, nil
		}
	}
	return nil, nil
}

// CheckSession inspeciona a disponibilidade do Chrome isolado e o estado de autenticação no NotebookLM.
func CheckSession(info *platform.Info, port int) Status {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}

	st := Status{
		Port: port,
	}

	chromeSt := profile.CheckStatusOnPort(info.BrowserProfile, port)
	st.ChromeRunning = chromeSt.IsRunning
	if !st.ChromeRunning {
		st.Message = fmt.Sprintf("Chrome DevTools inativo na porta %d. Inicie com 'agyo browser start' ou 'agyo notebook open'.", port)
		return st
	}

	tab, err := FindNotebookLMTab(port)
	if err != nil {
		st.Message = fmt.Sprintf("Falha ao inspecionar abas do Chrome: %v", err)
		return st
	}

	if tab == nil {
		st.HasTab = false
		st.Message = "Nenhuma aba do NotebookLM aberta no Chrome isolado. Execute 'agyo notebook open' para acessar."
		return st
	}

	st.HasTab = true
	st.TabID = tab.ID
	st.ActiveURL = tab.URL

	parsedURL, _ := url.Parse(tab.URL)
	host := ""
	path := ""
	if parsedURL != nil {
		host = parsedURL.Hostname()
		path = parsedURL.Path
	}

	// Se a aba estiver em tela de login ou apresentação inicial
	if host == "accounts.google.com" || strings.Contains(tab.URL, "ServiceLogin") || strings.Contains(path, "trynow") {
		st.IsLoggedIn = false
		st.Message = "Aba do NotebookLM aberta, porém aguardando autenticação na sua conta Google."
		return st
	}

	if host == "notebook.google.com" || host == "notebooklm.google.com" || strings.Contains(host, "notebooklm") {
		st.IsLoggedIn = true
		st.Message = "Sessão do NotebookLM conectada e autenticada com sucesso."
		return st
	}

	st.Message = fmt.Sprintf("Aba identificada em URL externa: %s", tab.URL)
	return st
}

// OpenSession garante que o Chrome isolado esteja aberto e navegue até o NotebookLM.
func OpenSession(info *platform.Info, port int) (*profile.Tab, error) {
	if port <= 0 {
		port = profile.DefaultDebugPort
	}

	// 1. Se Chrome não estiver rodando, inicia
	chromeSt := profile.CheckStatusOnPort(info.BrowserProfile, port)
	if !chromeSt.IsRunning {
		if err := profile.Start(info, profile.StartOptions{Port: port, ForceHeadless: false}); err != nil {
			return nil, fmt.Errorf("falha ao iniciar Chrome isolado: %w", err)
		}
	}

	// 2. Garante a aba do NotebookLM
	tab, err := FindNotebookLMTab(port)
	if err != nil {
		return nil, fmt.Errorf("falha ao verificar abas do Chrome: %w", err)
	}
	if tab != nil {
		return tab, nil
	}

	tab, err = profile.OpenTab(port, NotebookLMBaseURL)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir aba do NotebookLM: %w", err)
	}

	return tab, nil
}
