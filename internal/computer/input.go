package computer

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// InputEngine executa injeções de eventos HID (mouse e teclado).
type InputEngine struct {
	goos       string
	customExec func(ctx context.Context, name string, args ...string) ([]byte, error)
	inspector  *Inspector
}

// NewInputEngine cria um novo motor de injeção de input.
func NewInputEngine(ins *Inspector) *InputEngine {
	if ins == nil {
		ins = NewInspector()
	}
	return &InputEngine{
		inspector: ins,
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			return cmd.CombinedOutput()
		},
	}
}

func (ie *InputEngine) targetOS() string {
	if ie.goos != "" {
		return ie.goos
	}
	return runtime.GOOS
}

// ResolveActionCoordinates resolve coordenadas (x, y) caso a ação use uma referência de elemento (Ref).
func (ie *InputEngine) ResolveActionCoordinates(ctx context.Context, action *Action) error {
	if action.Ref == "" {
		return nil
	}
	snap, err := ie.inspector.InspectWindow(ctx)
	if err != nil {
		return fmt.Errorf("falha ao inspecionar janela para resolver ref %q: %w", action.Ref, err)
	}
	el := snap.FindElementByRef(action.Ref)
	if el == nil {
		return fmt.Errorf("elemento com ref %q não encontrado na janela ativa", action.Ref)
	}
	cx, cy := el.Bounds.Center()
	action.X = cx
	action.Y = cy
	return nil
}

// Execute dispara uma ação pontual no sistema operacional.
func (ie *InputEngine) Execute(ctx context.Context, action Action) error {
	if err := ValidateAction(action); err != nil {
		return err
	}

	if err := ie.ResolveActionCoordinates(ctx, &action); err != nil {
		return err
	}

	if ie.targetOS() == "darwin" {
		return ie.executeDarwin(ctx, action)
	}

	return ie.executeFallback(ctx, action)
}

// ExecuteBatch executa sequencialmente uma lista de ações em lote (macro).
func (ie *InputEngine) ExecuteBatch(ctx context.Context, actions []Action) error {
	for i, a := range actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ie.Execute(ctx, a); err != nil {
			return fmt.Errorf("falha ao executar ação %d (%s): %w", i+1, a.Type, err)
		}

		delay := a.DelayMs
		if delay <= 0 {
			delay = 50 // Delay padrão de 50ms para estabilização da interface
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

func (ie *InputEngine) executeDarwin(ctx context.Context, action Action) error {
	switch action.Type {
	case ActionClick:
		return ie.mouseClickDarwin(ctx, action.X, action.Y, 1, false)
	case ActionDoubleClick:
		return ie.mouseClickDarwin(ctx, action.X, action.Y, 2, false)
	case ActionRightClick:
		return ie.mouseClickDarwin(ctx, action.X, action.Y, 1, true)
	case ActionMove:
		return ie.mouseMoveDarwin(ctx, action.X, action.Y)
	case ActionDrag:
		return ie.mouseDragDarwin(ctx, action.X, action.Y, action.EndX, action.EndY)
	case ActionTypeString:
		return ie.typeTextDarwin(ctx, action.Text)
	case ActionKey, ActionShortcut:
		return ie.keyPressDarwin(ctx, action.Key)
	default:
		return fmt.Errorf("ação darwin não suportada: %s", action.Type)
	}
}

func (ie *InputEngine) mouseClickDarwin(ctx context.Context, x, y, clickCount int, rightClick bool) error {
	btnDown := "$.kCGEventLeftMouseDown"
	btnUp := "$.kCGEventLeftMouseUp"
	btnType := "$.kCGMouseButtonLeft"
	if rightClick {
		btnDown = "$.kCGEventRightMouseDown"
		btnUp = "$.kCGEventRightMouseUp"
		btnType = "$.kCGMouseButtonRight"
	}

	js := fmt.Sprintf(`
ObjC.import('CoreGraphics');
var pt = $.CGPointMake(%d, %d);
var move = $.CGEventCreateMouseEvent(null, $.kCGEventMouseMoved, pt, %s);
var down = $.CGEventCreateMouseEvent(null, %s, pt, %s);
var up = $.CGEventCreateMouseEvent(null, %s, pt, %s);
if (%d > 1) {
    $.CGEventSetIntegerValueField(down, $.kCGMouseEventClickState, %d);
    $.CGEventSetIntegerValueField(up, $.kCGMouseEventClickState, %d);
}
$.CGEventPost($.kCGHIDEventTap, move);
$.CGEventPost($.kCGHIDEventTap, down);
$.CGEventPost($.kCGHIDEventTap, up);
`, x, y, btnType, btnDown, btnType, btnUp, btnType, clickCount, clickCount, clickCount)

	_, err := ie.customExec(ctx, "osascript", "-l", "JavaScript", "-e", js)
	return err
}

func (ie *InputEngine) mouseMoveDarwin(ctx context.Context, x, y int) error {
	js := fmt.Sprintf(`
ObjC.import('CoreGraphics');
var pt = $.CGPointMake(%d, %d);
var move = $.CGEventCreateMouseEvent(null, $.kCGEventMouseMoved, pt, $.kCGMouseButtonLeft);
$.CGEventPost($.kCGHIDEventTap, move);
`, x, y)

	_, err := ie.customExec(ctx, "osascript", "-l", "JavaScript", "-e", js)
	return err
}

func (ie *InputEngine) mouseDragDarwin(ctx context.Context, startX, startY, endX, endY int) error {
	js := fmt.Sprintf(`
ObjC.import('CoreGraphics');
var start = $.CGPointMake(%d, %d);
var end = $.CGPointMake(%d, %d);
var moveStart = $.CGEventCreateMouseEvent(null, $.kCGEventMouseMoved, start, $.kCGMouseButtonLeft);
var down = $.CGEventCreateMouseEvent(null, $.kCGEventLeftMouseDown, start, $.kCGMouseButtonLeft);
var drag = $.CGEventCreateMouseEvent(null, $.kCGEventLeftMouseDragged, end, $.kCGMouseButtonLeft);
var up = $.CGEventCreateMouseEvent(null, $.kCGEventLeftMouseUp, end, $.kCGMouseButtonLeft);
$.CGEventPost($.kCGHIDEventTap, moveStart);
$.CGEventPost($.kCGHIDEventTap, down);
$.CGEventPost($.kCGHIDEventTap, drag);
$.CGEventPost($.kCGHIDEventTap, up);
`, startX, startY, endX, endY)

	_, err := ie.customExec(ctx, "osascript", "-l", "JavaScript", "-e", js)
	return err
}

func (ie *InputEngine) typeTextDarwin(ctx context.Context, text string) error {
	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)

	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
	_, err := ie.customExec(ctx, "osascript", "-e", script)
	return err
}

func (ie *InputEngine) keyPressDarwin(ctx context.Context, keyCombo string) error {
	parts := strings.Split(strings.ToLower(keyCombo), "+")
	var modifiers []string
	var mainKey string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "cmd", "command":
			modifiers = append(modifiers, "command down")
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "alt", "option", "opt":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		default:
			mainKey = p
		}
	}

	usingClause := ""
	if len(modifiers) > 0 {
		usingClause = " using {" + strings.Join(modifiers, ", ") + "}"
	}

	// Mapeia códigos de teclas especiais do teclado Apple
	var script string
	switch mainKey {
	case "return", "enter":
		script = fmt.Sprintf(`tell application "System Events" to key code 36%s`, usingClause)
	case "escape", "esc":
		script = fmt.Sprintf(`tell application "System Events" to key code 53%s`, usingClause)
	case "tab":
		script = fmt.Sprintf(`tell application "System Events" to key code 48%s`, usingClause)
	case "space":
		script = fmt.Sprintf(`tell application "System Events" to key code 49%s`, usingClause)
	case "backspace", "delete":
		script = fmt.Sprintf(`tell application "System Events" to key code 51%s`, usingClause)
	case "up":
		script = fmt.Sprintf(`tell application "System Events" to key code 126%s`, usingClause)
	case "down":
		script = fmt.Sprintf(`tell application "System Events" to key code 125%s`, usingClause)
	case "left":
		script = fmt.Sprintf(`tell application "System Events" to key code 123%s`, usingClause)
	case "right":
		script = fmt.Sprintf(`tell application "System Events" to key code 124%s`, usingClause)
	default:
		// Caractere padrão
		escaped := strings.ReplaceAll(mainKey, `"`, `\"`)
		script = fmt.Sprintf(`tell application "System Events" to keystroke "%s"%s`, escaped, usingClause)
	}

	_, err := ie.customExec(ctx, "osascript", "-e", script)
	return err
}

func (ie *InputEngine) executeFallback(ctx context.Context, action Action) error {
	// Fallback para ambientes não-darwin (ex: Linux com xdotool se disponível)
	if path, err := exec.LookPath("xdotool"); err == nil {
		switch action.Type {
		case ActionClick:
			_, err = ie.customExec(ctx, path, "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y), "click", "1")
			return err
		case ActionDoubleClick:
			_, err = ie.customExec(ctx, path, "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y), "click", "--repeat", "2", "1")
			return err
		case ActionRightClick:
			_, err = ie.customExec(ctx, path, "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y), "click", "3")
			return err
		case ActionMove:
			_, err = ie.customExec(ctx, path, "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y))
			return err
		case ActionTypeString:
			_, err = ie.customExec(ctx, path, "type", action.Text)
			return err
		case ActionKey, ActionShortcut:
			_, err = ie.customExec(ctx, path, "key", action.Key)
			return err
		}
	}
	return nil
}
