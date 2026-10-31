package computer

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
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

// ContentBlock modela blocos de conteúdo retornados pelo MCP (texto ou imagem).
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ToolCallResult representa a resposta canônica de uma ferramenta MCP.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// ServeMCP inicia o servidor MCP stdio de Computer Use.
func ServeMCP(engine Engine, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
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

		handleRequest(engine, out, &req)
	}

	return scanner.Err()
}

func handleRequest(engine Engine, out io.Writer, req *JSONRPCRequest) {
	if req.ID == nil {
		return // Notificação ignorada
	}

	switch req.Method {
	case "initialize":
		sendResult(out, req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "agyo-computer",
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

		executeTool(engine, out, req.ID, params.Name, params.Arguments)

	default:
		sendError(out, req.ID, -32601, fmt.Sprintf("Método desconhecido: %s", req.Method))
	}
}

func getToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "computer_inspect",
			"description": "Inspeciona a janela ativa no sistema operacional e retorna sua árvore semântica (botões, campos de texto, coordenadas e IDs ref) em texto ultra-compacto (~200 tokens). Use preferencialmente para não gastar tokens com screenshots.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "computer_action",
			"description": "Executa uma ação de mouse ou teclado no desktop. Permite clicar diretamente pelo ref do elemento (ex: 'ref_1') ou por coordenadas (x, y), digitar texto ou acionar atalhos de teclado.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"click", "double_click", "right_click", "move", "drag", "type", "key", "shortcut"},
						"description": "Tipo de ação a executar",
					},
					"ref": map[string]interface{}{
						"type":        "string",
						"description": "Identificador do elemento obtido em computer_inspect (ex: ref_1)",
					},
					"x": map[string]interface{}{
						"type":        "integer",
						"description": "Coordenada X (se ref não for utilizado)",
					},
					"y": map[string]interface{}{
						"type":        "integer",
						"description": "Coordenada Y (se ref não for utilizado)",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Texto a ser digitado para a ação type",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Tecla ou combinação para ação key/shortcut (ex: 'return', 'cmd+space')",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			"name":        "computer_batch",
			"description": "Executa uma macro (sequência de ações de mouse/teclado) de forma atômica em lote em um único turno, economizando latência e idas e vindas de rede.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"actions": map[string]interface{}{
						"type":        "array",
						"description": "Lista de ações para execução sequencial",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
				},
				"required": []string{"actions"},
			},
		},
		{
			"name":        "computer_shot",
			"description": "Captura de tela comprimida e redimensionada da janela ativa ou desktop. Use apenas quando a inspeção semântica não for suficiente (interfaces customizadas, canvas, gráficos).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Se true, captura apenas a janela ativa em vez da tela inteira (padrão true)",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"jpeg", "png"},
						"description": "Formato da imagem comprimida (padrão jpeg)",
					},
					"max_width": map[string]interface{}{
						"type":        "integer",
						"description": "Largura máxima de redimensionamento em pixels (padrão 1280)",
					},
					"quality": map[string]interface{}{
						"type":        "integer",
						"description": "Qualidade da compressão de 1 a 100 (padrão 75)",
					},
				},
			},
		},
	}
}

func executeTool(engine Engine, out io.Writer, id interface{}, name string, args map[string]interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch name {
	case "computer_inspect":
		snap, err := engine.Inspect(ctx)
		if err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao inspecionar janela: %v", err))
			return
		}
		sendToolText(out, id, snap.ToText())

	case "computer_action":
		actionType, _ := args["action"].(string)
		ref, _ := args["ref"].(string)
		text, _ := args["text"].(string)
		key, _ := args["key"].(string)

		x := -1
		if val, ok := args["x"].(float64); ok {
			x = int(val)
		}
		y := -1
		if val, ok := args["y"].(float64); ok {
			y = int(val)
		}

		action := Action{
			Type: ActionType(actionType),
			Ref:  ref,
			X:    x,
			Y:    y,
			Text: text,
			Key:  key,
		}

		if err := engine.Execute(ctx, action); err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro ao executar ação %s: %v", actionType, err))
			return
		}
		sendToolText(out, id, fmt.Sprintf("Ação %s executada com sucesso.", actionType))

	case "computer_batch":
		rawActions, ok := args["actions"].([]interface{})
		if !ok || len(rawActions) == 0 {
			sendToolError(out, id, "A lista de ações 'actions' está vazia ou é inválida")
			return
		}

		var actions []Action
		for _, item := range rawActions {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			actType, _ := itemMap["action"].(string)
			ref, _ := itemMap["ref"].(string)
			text, _ := itemMap["text"].(string)
			key, _ := itemMap["key"].(string)
			x := -1
			if val, ok := itemMap["x"].(float64); ok {
				x = int(val)
			}
			y := -1
			if val, ok := itemMap["y"].(float64); ok {
				y = int(val)
			}

			actions = append(actions, Action{
				Type: ActionType(actType),
				Ref:  ref,
				X:    x,
				Y:    y,
				Text: text,
				Key:  key,
			})
		}

		if err := engine.ExecuteBatch(ctx, actions); err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro no lote de ações: %v", err))
			return
		}
		sendToolText(out, id, fmt.Sprintf("Macro com %d ações executada com sucesso.", len(actions)))

	case "computer_shot":
		opts := ShotOptions{
			WindowOnly: true,
			Format:     "jpeg",
			MaxWidth:   1280,
			Quality:    75,
		}
		if wo, ok := args["window_only"].(bool); ok {
			opts.WindowOnly = wo
		}
		if fmtVal, ok := args["format"].(string); ok && fmtVal != "" {
			opts.Format = fmtVal
		}
		if mw, ok := args["max_width"].(float64); ok && mw > 0 {
			opts.MaxWidth = int(mw)
		}
		if q, ok := args["quality"].(float64); ok && q > 0 {
			opts.Quality = int(q)
		}

		data, path, err := engine.Capture(ctx, opts)
		if err != nil {
			sendToolError(out, id, fmt.Sprintf("Erro na captura de tela: %v", err))
			return
		}

		mimeType := "image/jpeg"
		if strings.EqualFold(opts.Format, "png") {
			mimeType = "image/png"
		}

		b64Data := base64.StdEncoding.EncodeToString(data)
		sendToolImageAndText(out, id, b64Data, mimeType, fmt.Sprintf("Captura salva em %s (%d bytes)", path, len(data)))

	default:
		sendToolError(out, id, fmt.Sprintf("Ferramenta desconhecida: %s", name))
	}
}

func sendToolText(out io.Writer, id interface{}, text string) {
	sendResult(out, id, ToolCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
		IsError: false,
	})
}

func sendToolImageAndText(out io.Writer, id interface{}, b64Image, mimeType, text string) {
	sendResult(out, id, ToolCallResult{
		Content: []ContentBlock{
			{Type: "image", Data: b64Image, MimeType: mimeType},
			{Type: "text", Text: text},
		},
		IsError: false,
	})
}

func sendToolError(out io.Writer, id interface{}, msg string) {
	sendResult(out, id, ToolCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Erro: " + msg},
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
	fmt.Fprintf(out, "%s\n", string(bytes))
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
	fmt.Fprintf(out, "%s\n", string(bytes))
}
