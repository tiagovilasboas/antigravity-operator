package computer

import (
	"context"
	"strings"
	"testing"
)

func TestInputEngine_ResolveActionCoordinates(t *testing.T) {
	mockJSON := `{"app":"App","elements":[{"role":"AXButton","title":"ClickMe","x":100,"y":200,"w":50,"h":40}]}`
	ins := &Inspector{
		goos: "darwin",
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(mockJSON), nil
		},
	}

	ie := &InputEngine{
		goos:      "darwin",
		inspector: ins,
	}
	action := Action{Type: ActionClick, Ref: "ref_1"}

	if err := ie.ResolveActionCoordinates(context.Background(), &action); err != nil {
		t.Fatalf("ResolveActionCoordinates error: %v", err)
	}

	// Center of X: 100, Y: 200, W: 50, H: 40 -> Center = (125, 220)
	if action.X != 125 || action.Y != 220 {
		t.Errorf("got coordinates (%d, %d); want (125, 220)", action.X, action.Y)
	}
}

func TestInputEngine_ExecuteDarwin_Mock(t *testing.T) {
	var executed []string
	ie := &InputEngine{
		goos: "darwin",
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executed = append(executed, name+" "+strings.Join(args, " "))
			return []byte("ok"), nil
		},
		inspector: NewInspector(),
	}

	// 1. Click
	err := ie.Execute(context.Background(), Action{Type: ActionClick, X: 50, Y: 60})
	if err != nil {
		t.Fatalf("Click error: %v", err)
	}
	if len(executed) != 1 || !strings.Contains(executed[0], "CGPointMake(50, 60)") {
		t.Errorf("Click executed mismatch: %v", executed)
	}

	// 2. Type Text
	executed = nil
	err = ie.Execute(context.Background(), Action{Type: ActionTypeString, Text: "Antigravity"})
	if err != nil {
		t.Fatalf("Type error: %v", err)
	}
	if len(executed) != 1 || !strings.Contains(executed[0], `keystroke "Antigravity"`) {
		t.Errorf("Type executed mismatch: %v", executed)
	}

	// 3. Shortcut cmd+space
	executed = nil
	err = ie.Execute(context.Background(), Action{Type: ActionShortcut, Key: "cmd+space"})
	if err != nil {
		t.Fatalf("Shortcut error: %v", err)
	}
	if len(executed) != 1 || !strings.Contains(executed[0], "key code 49 using {command down}") {
		t.Errorf("Shortcut executed mismatch: %v", executed)
	}
}

func TestInputEngine_ExecuteBatch(t *testing.T) {
	count := 0
	ie := &InputEngine{
		goos: "darwin",
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			count++
			return []byte("ok"), nil
		},
		inspector: NewInspector(),
	}

	batch := []Action{
		{Type: ActionClick, X: 10, Y: 10, DelayMs: 1},
		{Type: ActionTypeString, Text: "Test", DelayMs: 1},
		{Type: ActionKey, Key: "return", DelayMs: 1},
	}

	err := ie.ExecuteBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("ExecuteBatch error: %v", err)
	}
	if count != 3 {
		t.Errorf("executed %d actions; want 3", count)
	}
}
