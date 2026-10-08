package notebook

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

const (
	NotebookLMBaseURL = "https://notebooklm.google.com"
	NotebookLMDomain  = "notebooklm.google.com"
)

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

	tab, err := profile.FindTab(port, NotebookLMDomain)
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

	// Se a aba estiver em tela de login ou autenticação do Google
	if strings.Contains(tab.URL, "accounts.google.com") || strings.Contains(tab.URL, "ServiceLogin") {
		st.IsLoggedIn = false
		st.Message = "Aba do NotebookLM aberta, porém aguardando autenticação na sua conta Google."
		return st
	}

	if strings.Contains(tab.URL, NotebookLMDomain) {
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
	tab, err := profile.EnsureTab(port, NotebookLMBaseURL, NotebookLMDomain)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir aba do NotebookLM: %w", err)
	}

	return tab, nil
}
