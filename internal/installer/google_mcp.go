package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

const GoogleCalendarMCPServerName = "google-calendar"

// MCPServerStatusInfo descreve o estado de registro do servidor MCP no Antigravity.
type MCPServerStatusInfo struct {
	Name         string   `json:"name"`
	Installed    bool     `json:"installed"`
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	SchemaDir    string   `json:"schema_dir,omitempty"`
	SchemasCount int      `json:"schemas_count,omitempty"`
}

// Schemas do Google Calendar e Workspace para o Antigravity (Lazy Tools)
var calendarToolSchemas = map[string]string{
	"google_calendar_list_events.json":  `{"name":"google_calendar_list_events","description":"Lista eventos agendados no Google Calendar do usuário (reuniões, prazos, blocos de foco). 100% autônomo via API em background, zero abas no navegador.","parameters":{"properties":{"max_results":{"description":"Número máximo de eventos a retornar (padrão 20)","type":"integer"},"time_max":{"description":"Data/hora limite em formato RFC3339 ou YYYY-MM-DD","type":"string"},"time_min":{"description":"Data/hora de início em formato RFC3339 ou YYYY-MM-DD (padrão: agora)","type":"string"}},"type":"object"}}`,
	"google_calendar_create_event.json": `{"name":"google_calendar_create_event","description":"Agenda um novo evento no Google Calendar do usuário diretamente via API REST em background (sem abrir Chrome nem abas).","parameters":{"properties":{"description":{"description":"Descrição detalhada do evento","type":"string"},"end_time":{"description":"Horário de término (RFC3339, YYYY-MM-DD HH:MM, ou expressões como 'hoje 18:00', '+1h')","type":"string"},"location":{"description":"Localização física ou link de videoconferência","type":"string"},"start_time":{"description":"Horário de início (RFC3339, YYYY-MM-DD HH:MM, ou expressões como 'hoje 14:00', 'amanhã 10:00')","type":"string"},"summary":{"description":"Título do evento","type":"string"}},"required":["summary","start_time"],"type":"object"}}`,
	"google_docs_read.json":             `{"name":"google_docs_read","description":"Lê o conteúdo textual estruturado de um documento do Google Docs diretamente via API em background.","parameters":{"properties":{"document_id":{"description":"ID ou URL do Google Doc a ser lido","type":"string"}},"required":["document_id"],"type":"object"}}`,
	"google_docs_create.json":           `{"name":"google_docs_create","description":"Cria um novo documento no Google Docs via API em background com título e conteúdo inicial.","parameters":{"properties":{"content":{"description":"Conteúdo textual inicial a ser gravado no documento","type":"string"},"title":{"description":"Título do novo documento","type":"string"}},"required":["title"],"type":"object"}}`,
}

// InstallCalendarMCP registra o servidor MCP do Google Calendar no Antigravity e grava os schemas JSON.
func InstallCalendarMCP(info *platform.Info) (*MCPServerStatusInfo, error) {
	geminiDir := info.GeminiDir
	if geminiDir == "" {
		geminiDir = filepath.Join(info.HomeDir, ".gemini", "antigravity")
	}

	// 1. Registra no default-servers.json (GeminiDir/mcp/default-servers.json)
	defaultServersPath := filepath.Join(geminiDir, "mcp", "default-servers.json")
	_ = registerServerInManifest(defaultServersPath, GoogleCalendarMCPServerName, "agyo", []string{"google", "mcp"})

	// 2. Registra no mcp_config.json (~/.gemini/config/mcp_config.json)
	userConfigPath := filepath.Join(info.HomeDir, ".gemini", "config", "mcp_config.json")
	_ = registerServerInManifest(userConfigPath, GoogleCalendarMCPServerName, "agyo", []string{"google", "mcp"})

	// 3. Grava schemas de ferramentas em geminiDir/mcp/google-calendar/
	schemaDir := filepath.Join(geminiDir, "mcp", GoogleCalendarMCPServerName)
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return nil, fmt.Errorf("criar pasta de schemas %s: %w", schemaDir, err)
	}

	for filename, content := range calendarToolSchemas {
		filePath := filepath.Join(schemaDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("gravar schema %s: %w", filePath, err)
		}
	}

	return &MCPServerStatusInfo{
		Name:         GoogleCalendarMCPServerName,
		Installed:    true,
		Command:      "agyo",
		Args:         []string{"google", "mcp"},
		SchemaDir:    schemaDir,
		SchemasCount: len(calendarToolSchemas),
	}, nil
}

// CalendarMCPStatus verifica se o servidor MCP está registrado no Antigravity.
func CalendarMCPStatus(info *platform.Info) (*MCPServerStatusInfo, error) {
	geminiDir := info.GeminiDir
	if geminiDir == "" {
		geminiDir = filepath.Join(info.HomeDir, ".gemini", "antigravity")
	}

	schemaDir := filepath.Join(geminiDir, "mcp", GoogleCalendarMCPServerName)
	defaultServersPath := filepath.Join(geminiDir, "mcp", "default-servers.json")
	userConfigPath := filepath.Join(info.HomeDir, ".gemini", "config", "mcp_config.json")

	isRegDefault := isServerInManifest(defaultServersPath, GoogleCalendarMCPServerName)
	isRegUser := isServerInManifest(userConfigPath, GoogleCalendarMCPServerName)

	installed := isRegDefault || isRegUser

	count := 0
	if entries, err := os.ReadDir(schemaDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				count++
			}
		}
	}

	return &MCPServerStatusInfo{
		Name:         GoogleCalendarMCPServerName,
		Installed:    installed,
		Command:      "agyo",
		Args:         []string{"google", "mcp"},
		SchemaDir:    schemaDir,
		SchemasCount: count,
	}, nil
}

// UninstallCalendarMCP desregistra o servidor MCP e remove os schemas.
func UninstallCalendarMCP(info *platform.Info) error {
	geminiDir := info.GeminiDir
	if geminiDir == "" {
		geminiDir = filepath.Join(info.HomeDir, ".gemini", "antigravity")
	}

	defaultServersPath := filepath.Join(geminiDir, "mcp", "default-servers.json")
	_ = deregisterServerInManifest(defaultServersPath, GoogleCalendarMCPServerName)

	userConfigPath := filepath.Join(info.HomeDir, ".gemini", "config", "mcp_config.json")
	_ = deregisterServerInManifest(userConfigPath, GoogleCalendarMCPServerName)

	schemaDir := filepath.Join(geminiDir, "mcp", GoogleCalendarMCPServerName)
	_ = os.RemoveAll(schemaDir)

	return nil
}

func registerServerInManifest(manifestPath, serverName, command string, args []string) error {
	_ = os.MkdirAll(filepath.Dir(manifestPath), 0755)

	data, err := os.ReadFile(manifestPath)
	var cfg struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]interface{})
	}

	cfg.MCPServers[serverName] = map[string]interface{}{
		"command": command,
		"args":    args,
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, out, 0644)
}

func isServerInManifest(manifestPath, serverName string) bool {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var cfg struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	_, ok := cfg.MCPServers[serverName]
	return ok
}

func deregisterServerInManifest(manifestPath, serverName string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	if cfg.MCPServers == nil {
		return nil
	}

	if _, ok := cfg.MCPServers[serverName]; ok {
		delete(cfg.MCPServers, serverName)
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(manifestPath, out, 0644)
	}
	return nil
}
