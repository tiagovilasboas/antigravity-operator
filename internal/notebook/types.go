package notebook

// Notebook representa um caderno do Google NotebookLM.
type Notebook struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	SourceCount int    `json:"source_count,omitempty"`
}

// Status sintetiza a situação de conexão e autenticação com o NotebookLM.
type Status struct {
	ChromeRunning bool   `json:"chrome_running"`
	Port          int    `json:"port"`
	HasTab        bool   `json:"has_tab"`
	TabID         string `json:"tab_id,omitempty"`
	IsLoggedIn    bool   `json:"is_logged_in"`
	ActiveURL     string `json:"active_url,omitempty"`
	Message       string `json:"message"`
}

// AskResult representa a resposta estruturada de uma consulta a um caderno.
type AskResult struct {
	NotebookID string   `json:"notebook_id"`
	Query      string   `json:"query"`
	Answer     string   `json:"answer"`
	Citations  []string `json:"citations,omitempty"`
}
