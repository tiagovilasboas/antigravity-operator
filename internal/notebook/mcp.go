package notebook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

// JSONRPCRequest modela uma requisição JSON-RPC 2.0 do MCP.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse modela uma resposta JSON-RPC 2.0.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError detalha erros no protocolo JSON-RPC.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolContent representa a carga de texto do resultado de uma ferramenta.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResult representa a resposta canônica de uma ferramenta MCP.
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError"`
}

// ServeMCP inicia um servidor MCP stdio atendendo chamadas do Google Antigravity e outros agentes.
func ServeMCP(info *platform.Info, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Permite payloads de até 10MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(out, nil, -32700, "Parse error")
			continue
		}

		handleRequest(info, out, &req)
	}

	return scanner.Err()
}

func handleRequest(info *platform.Info, out io.Writer, req *JSONRPCRequest) {
	// Se for notificação (sem ID), não enviamos resposta
	if req.ID == nil {
		return
	}

	switch req.Method {
	case "initialize":
		sendResult(out, req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "agyo-notebooklm",
				"version": "0.5.0",
			},
		})

	case "ping":
		sendResult(out, req.ID, map[string]interface{}{})

	case "tools/list":
		sendResult(out, req.ID, map[string]interface{}{
			"tools": getToolDefinitions(),
		})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendToolError(out, req.ID, fmt.Sprintf("Parâmetros inválidos: %v", err))
			return
		}

		executeTool(info, out, req.ID, params.Name, params.Arguments)

	default:
		sendError(out, req.ID, -32601, fmt.Sprintf("Método desconhecido: %s", req.Method))
	}
}

func getToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "notebooklm_status",
			"description": "Verifica o status da conexão e autenticação com o Google NotebookLM via Chrome isolado.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "notebooklm_list",
			"description": "Lista todos os cadernos disponíveis no Google NotebookLM para a conta autenticada.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "notebooklm_ask",
			"description": "Faz uma pergunta ou consulta fundamentada a um caderno específico do Google NotebookLM.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"notebook_id": map[string]interface{}{
						"type":        "string",
						"description": "ID ou hash do caderno no NotebookLM (ex: 1234abcd-...)",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Pergunta ou comando de pesquisa com base nas fontes do caderno",
					},
				},
				"required": []string{"notebook_id", "query"},
			},
		},
		{
			"name":        "notebooklm_push",
			"description": "Adiciona uma nova nota ou fonte em texto/markdown a um caderno do Google NotebookLM.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"notebook_id": map[string]interface{}{
						"type":        "string",
						"description": "ID do caderno alvo",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Título da nota ou fonte",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Conteúdo em texto ou markdown",
					},
				},
				"required": []string{"notebook_id", "title", "content"},
			},
		},
		{
			"name":        "notebooklm_add_source",
			"description": "Adiciona uma URL de site ou vídeo do YouTube diretamente como fonte no Google NotebookLM sem abrir o player de vídeo.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"notebook_id": map[string]interface{}{
						"type":        "string",
						"description": "ID do caderno alvo no NotebookLM",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL do vídeo do YouTube ou página web a ser indexada",
					},
				},
				"required": []string{"notebook_id", "url"},
			},
		},
	}
}

func executeTool(info *platform.Info, out io.Writer, id interface{}, name string, args map[string]interface{}) {
	switch name {
	case "notebooklm_status":
		st := CheckSession(info, 0)
		resJSON, _ := json.MarshalIndent(st, "", "  ")
		sendToolSuccess(out, id, string(resJSON))

	case "notebooklm_list":
		notebooks, err := ListNotebooks(info, 0)
		if err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao listar cadernos: %v", err))
			return
		}
		if len(notebooks) == 0 {
			sendToolSuccess(out, id, "Nenhum caderno encontrado na tela inicial do NotebookLM. Verifique se você está autenticado ou se possui cadernos criados.")
			return
		}
		resJSON, _ := json.MarshalIndent(notebooks, "", "  ")
		sendToolSuccess(out, id, string(resJSON))

	case "notebooklm_ask":
		notebookID, _ := args["notebook_id"].(string)
		query, _ := args["query"].(string)
		if notebookID == "" || query == "" {
			sendToolError(out, id, "Os campos 'notebook_id' e 'query' são obrigatórios.")
			return
		}
		res, err := AskNotebook(info, 0, notebookID, query)
		if err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao consultar caderno: %v", err))
			return
		}
		resJSON, _ := json.MarshalIndent(res, "", "  ")
		sendToolSuccess(out, id, string(resJSON))

	case "notebooklm_push":
		notebookID, _ := args["notebook_id"].(string)
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		if notebookID == "" || content == "" {
			sendToolError(out, id, "Os campos 'notebook_id' e 'content' são obrigatórios.")
			return
		}
		if err := PushSource(info, 0, notebookID, title, content); err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao adicionar nota: %v", err))
			return
		}
		sendToolSuccess(out, id, fmt.Sprintf("Nota '%s' adicionada com sucesso ao caderno %s.", title, notebookID))

	case "notebooklm_add_source":
		notebookID, _ := args["notebook_id"].(string)
		sourceURL, _ := args["url"].(string)
		if notebookID == "" || sourceURL == "" {
			sendToolError(out, id, "Os campos 'notebook_id' e 'url' são obrigatórios.")
			return
		}
		if err := AddSourceURL(info, 0, notebookID, sourceURL); err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao adicionar fonte: %v", err))
			return
		}
		sendToolSuccess(out, id, fmt.Sprintf("Fonte '%s' adicionada com sucesso ao caderno %s.", sourceURL, notebookID))

	default:
		sendToolError(out, id, fmt.Sprintf("Ferramenta '%s' não suportada.", name))
	}
}

func sendToolSuccess(out io.Writer, id interface{}, text string) {
	sendResult(out, id, ToolCallResult{
		Content: []ToolContent{
			{Type: "text", Text: text},
		},
		IsError: false,
	})
}

func sendToolError(out io.Writer, id interface{}, text string) {
	sendResult(out, id, ToolCallResult{
		Content: []ToolContent{
			{Type: "text", Text: text},
		},
		IsError: true,
	})
}

func sendResult(out io.Writer, id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(out, "%s\n", bytes)
}

func sendError(out io.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(out, "%s\n", bytes)
}
