package computer

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RawElement modela a saída bruta do script de extração do sistema operacional.
type RawElement struct {
	Role  string `json:"role"`
	Title string `json:"title"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// RawSnapshot modela a carga JSON retornada pela automação do SO.
type RawSnapshot struct {
	App         string       `json:"app"`
	PID         int          `json:"pid"`
	WindowTitle string       `json:"window_title"`
	Bounds      Rect         `json:"bounds"`
	Elements    []RawElement `json:"elements"`
	Error       string       `json:"error,omitempty"`
}

// Inspector é responsável por extrair a árvore semântica da janela em foco.
type Inspector struct {
	customExec func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewInspector inicializa um Inspector de acessibilidade.
func NewInspector() *Inspector {
	return &Inspector{
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			return cmd.Output()
		},
	}
}

// InspectWindow extrai os elementos semânticos da janela ativa no SO.
func (ins *Inspector) InspectWindow(ctx context.Context) (*Snapshot, error) {
	if runtime.GOOS == "darwin" {
		return ins.inspectDarwin(ctx)
	}
	return ins.inspectFallback(ctx)
}

func (ins *Inspector) inspectDarwin(ctx context.Context) (*Snapshot, error) {
	const script = `
set elList to {}
tell application "System Events"
    if not (exists (first process whose frontmost is true)) then
        return "{\"error\":\"Nenhum processo em primeiro plano\"}"
    end if
    set frontProc to first process whose frontmost is true
    set pName to name of frontProc
    set pId to unix id of frontProc
    set wTitle to ""
    set wx to 0
    set wy to 0
    set ww to 0
    set wh to 0
    if (count of windows of frontProc) > 0 then
        set w to window 1 of frontProc
        try
            set wTitle to name of w
        end try
        try
            set {wx, wy} to position of w
            set {ww, wh} to size of w
        end try
        repeat with el in (UI elements of w)
            try
                set r to role of el
                set t to ""
                try
                    set t to name of el
                    if t is missing value then set t to ""
                end try
                set {ex, ey} to position of el
                set {ew, eh} to size of el
                set end of elList to "{\"role\":\"" & r & "\",\"title\":\"" & t & "\",\"x\":" & (ex as integer) & ",\"y\":" & (ey as integer) & ",\"w\":" & (ew as integer) & ",\"h\":" & (eh as integer) & "}"
            end try
        end repeat
    end if
end tell
set AppleScript's text item delimiters to ","
set elJson to "[" & (elList as string) & "]"
return "{\"app\":\"" & pName & "\",\"pid\":" & pId & ",\"window_title\":\"" & wTitle & "\",\"bounds\":{\"x\":" & (wx as integer) & ",\"y\":" & (wy as integer) & ",\"width\":" & (ww as integer) & ",\"height\":" & (wh as integer) & "},\"elements\":" & elJson & "}"
`
	out, err := ins.customExec(ctx, "osascript", "-e", script)
	if err != nil {
		return nil, fmt.Errorf("falha ao inspecionar acessibilidade via osascript: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, fmt.Errorf("retorno vazio da inspeção de acessibilidade")
	}

	var raw RawSnapshot
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de acessibilidade: %w (output: %q)", err, trimmed)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("erro reportado pelo SO: %s", raw.Error)
	}

	snap := &Snapshot{
		App:         raw.App,
		PID:         raw.PID,
		WindowTitle: raw.WindowTitle,
		Bounds:      raw.Bounds,
		Timestamp:   time.Now(),
		Elements:    make([]*Element, 0, len(raw.Elements)),
	}

	for i, el := range raw.Elements {
		// Normaliza campos "missing value" do AppleScript
		title := el.Title
		if title == "missing value" {
			title = ""
		}

		snap.Elements = append(snap.Elements, &Element{
			Ref:     fmt.Sprintf("ref_%d", i+1),
			Role:    el.Role,
			Title:   title,
			Enabled: true,
			Bounds: Rect{
				X:      el.X,
				Y:      el.Y,
				Width:  el.W,
				Height: el.H,
			},
		})
	}

	return snap, nil
}

func (ins *Inspector) inspectFallback(ctx context.Context) (*Snapshot, error) {
	// Em ambientes sem macOS AppleScript (ex: Linux Headless), retorna estrutura básica
	return &Snapshot{
		App:         "Desktop",
		PID:         0,
		WindowTitle: "Main",
		Bounds:      Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		Elements:    []*Element{},
		Timestamp:   time.Now(),
	}, nil
}

// ToText converte o Snapshot em um formato textual ultra-compacto, otimizado para consumo por LLMs (~100-300 tokens).
func (s *Snapshot) ToText() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("App: %q (PID: %d)\n", s.App, s.PID))
	sb.WriteString(fmt.Sprintf("Window: %q | Bounds: [%d, %d, %d, %d]\n", s.WindowTitle, s.Bounds.X, s.Bounds.Y, s.Bounds.Width, s.Bounds.Height))

	if len(s.Elements) == 0 {
		sb.WriteString("Elements: (nenhum elemento detectado na árvore semântica)\n")
		return sb.String()
	}

	sb.WriteString("Interactive Elements:\n")
	for _, el := range s.Elements {
		titlePart := ""
		if el.Title != "" {
			titlePart = fmt.Sprintf(" %q", el.Title)
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s%s at (%d, %d) [%dx%d]\n",
			el.Ref,
			el.Role,
			titlePart,
			el.Bounds.X,
			el.Bounds.Y,
			el.Bounds.Width,
			el.Bounds.Height,
		))
	}

	return sb.String()
}
