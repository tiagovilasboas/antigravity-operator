package computer

import (
	"context"
	"fmt"
	"time"
)

// Rect define uma caixa delimitadora na tela (coordenadas de tela do SO).
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Center retorna o ponto central (x, y) da caixa delimitadora.
func (r Rect) Center() (int, int) {
	return r.X + (r.Width / 2), r.Y + (r.Height / 2)
}

// Element representa um elemento semântico da interface (Accessibility Tree).
type Element struct {
	Ref      string     `json:"ref"`                // Identificador único na árvore (ex: ref_1)
	Role     string     `json:"role"`               // Papel do elemento (ex: AXButton, AXTextField)
	Title    string     `json:"title,omitempty"`    // Título ou label visível
	Value    string     `json:"value,omitempty"`    // Valor textual atual
	Bounds   Rect       `json:"bounds"`             // Posição e dimensões na tela
	Enabled  bool       `json:"enabled"`            // Se o elemento aceita interação
	Children []*Element `json:"children,omitempty"` // Filhos hierárquicos
}

// Snapshot consolida o estado semântico de uma janela/aplicativo inspecionado.
type Snapshot struct {
	App         string     `json:"app"`
	PID         int        `json:"pid"`
	WindowTitle string     `json:"window_title"`
	Bounds      Rect       `json:"bounds"`
	Elements    []*Element `json:"elements"`
	Timestamp   time.Time  `json:"timestamp"`
}

// FindElementByRef busca um elemento na árvore pelo identificador Ref.
func (s *Snapshot) FindElementByRef(ref string) *Element {
	for _, el := range s.Elements {
		if found := findRecursive(el, ref); found != nil {
			return found
		}
	}
	return nil
}

func findRecursive(el *Element, ref string) *Element {
	if el.Ref == ref {
		return el
	}
	for _, child := range el.Children {
		if found := findRecursive(child, ref); found != nil {
			return found
		}
	}
	return nil
}

// ActionType enumera as ações possíveis de interação com o sistema operacional.
type ActionType string

const (
	ActionClick       ActionType = "click"
	ActionDoubleClick ActionType = "double_click"
	ActionRightClick  ActionType = "right_click"
	ActionMove        ActionType = "move"
	ActionDrag        ActionType = "drag"
	ActionTypeString  ActionType = "type"
	ActionKey         ActionType = "key"
	ActionShortcut    ActionType = "shortcut"
)

// Action representa uma operação de mouse ou teclado a ser executada pelo engine.
type Action struct {
	Type    ActionType `json:"type"`
	Ref     string     `json:"ref,omitempty"`      // Referência do elemento (resolve coordenadas via Snapshot)
	X       int        `json:"x,omitempty"`        // Coordenada X absoluta (se Ref não for fornecido)
	Y       int        `json:"y,omitempty"`        // Coordenada Y absoluta (se Ref não for fornecido)
	EndX    int        `json:"end_x,omitempty"`    // Coordenada final X para drag
	EndY    int        `json:"end_y,omitempty"`    // Coordenada final Y para drag
	Text    string     `json:"text,omitempty"`     // Texto para ação de digitação
	Key     string     `json:"key,omitempty"`      // Tecla ou combinação (ex: "return", "cmd+space")
	DelayMs int        `json:"delay_ms,omitempty"` // Pausa após execução em milissegundos
}

// ShotOptions configura a captura visual de tela.
type ShotOptions struct {
	WindowOnly bool   `json:"window_only"` // Se true, captura apenas a janela ativa em vez da tela cheia
	Format     string `json:"format"`      // "jpeg" ou "png" (padrão "jpeg")
	MaxWidth   int    `json:"max_width"`   // Largura máxima de redimensionamento (ex: 1280)
	Quality    int    `json:"quality"`     // Qualidade da compressão 1-100 (padrão 75)
	OutPath    string `json:"out_path"`    // Arquivo de destino (opcional; se vazio gera em arquivo temporário)
}

// Engine define a interface de automação operacional de Computer Use.
type Engine interface {
	Inspect(ctx context.Context) (*Snapshot, error)
	Execute(ctx context.Context, action Action) error
	ExecuteBatch(ctx context.Context, actions []Action) error
	Capture(ctx context.Context, opts ShotOptions) ([]byte, string, error)
}

// DefaultEngine implementa a interface Engine integrando acessibilidade, input e captura.
type DefaultEngine struct {
	Inspector *Inspector
	Capturer  *ScreenCapturer
	Input     *InputEngine
}

// NewEngine inicializa a engine padrão do agyo.
func NewEngine() *DefaultEngine {
	ins := NewInspector()
	return &DefaultEngine{
		Inspector: ins,
		Capturer:  NewScreenCapturer(ins),
		Input:     NewInputEngine(ins),
	}
}

// Inspect extrai a árvore semântica da janela ativa.
func (e *DefaultEngine) Inspect(ctx context.Context) (*Snapshot, error) {
	return e.Inspector.InspectWindow(ctx)
}

// Execute dispara uma ação isolada de mouse ou teclado.
func (e *DefaultEngine) Execute(ctx context.Context, action Action) error {
	return e.Input.Execute(ctx, action)
}

// ExecuteBatch dispara uma sequência de ações em lote (macro).
func (e *DefaultEngine) ExecuteBatch(ctx context.Context, actions []Action) error {
	return e.Input.ExecuteBatch(ctx, actions)
}

// Capture tira uma captura de tela comprimida e downsampled.
func (e *DefaultEngine) Capture(ctx context.Context, opts ShotOptions) ([]byte, string, error) {
	return e.Capturer.Capture(ctx, opts)
}

// ValidateAction valida parâmetros mínimos antes do envio ao sistema operacional.
func ValidateAction(a Action) error {
	switch a.Type {
	case ActionClick, ActionDoubleClick, ActionRightClick, ActionMove:
		if a.Ref == "" && (a.X < 0 || a.Y < 0) {
			return fmt.Errorf("ação %q requer ref válida ou coordenadas (x >= 0, y >= 0)", a.Type)
		}
	case ActionDrag:
		if a.X < 0 || a.Y < 0 || a.EndX < 0 || a.EndY < 0 {
			return fmt.Errorf("ação drag requer coordenadas iniciais (x, y) e finais (end_x, end_y) >= 0")
		}
	case ActionTypeString:
		if a.Text == "" {
			return fmt.Errorf("ação type requer campo text preenchido")
		}
	case ActionKey, ActionShortcut:
		if a.Key == "" {
			return fmt.Errorf("ação %q requer campo key preenchido", a.Type)
		}
	default:
		return fmt.Errorf("tipo de ação desconhecido: %q", a.Type)
	}
	return nil
}
