package computer

import (
	"context"
	"strings"
	"testing"
)

func TestInspector_InspectDarwin_Mock(t *testing.T) {
	mockJSON := `{"app":"TestApp","pid":4242,"window_title":"Test Window","bounds":{"x":100,"y":50,"width":800,"height":600},"elements":[{"role":"AXButton","title":"Confirm","x":120,"y":70,"w":80,"h":30},{"role":"AXTextField","title":"missing value","x":220,"y":70,"w":150,"h":30}]}`

	ins := &Inspector{
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(mockJSON), nil
		},
	}

	snap, err := ins.inspectDarwin(context.Background())
	if err != nil {
		t.Fatalf("inspectDarwin() returned error: %v", err)
	}

	if snap.App != "TestApp" || snap.PID != 4242 {
		t.Errorf("got App=%q, PID=%d; want TestApp, 4242", snap.App, snap.PID)
	}

	if len(snap.Elements) != 2 {
		t.Fatalf("got %d elements; want 2", len(snap.Elements))
	}

	el1 := snap.Elements[0]
	if el1.Ref != "ref_1" || el1.Role != "AXButton" || el1.Title != "Confirm" {
		t.Errorf("el1 mismatch: %+v", el1)
	}

	el2 := snap.Elements[1]
	// "missing value" must be normalized to ""
	if el2.Ref != "ref_2" || el2.Title != "" {
		t.Errorf("el2 title not normalized: %+v", el2)
	}

	// Test ToText formatting
	txt := snap.ToText()
	if !strings.Contains(txt, `App: "TestApp"`) || !strings.Contains(txt, `[ref_1] AXButton "Confirm"`) {
		t.Errorf("ToText() output unexpected:\n%s", txt)
	}
}

func TestInspector_Fallback(t *testing.T) {
	ins := NewInspector()
	snap, err := ins.inspectFallback(context.Background())
	if err != nil {
		t.Fatalf("inspectFallback error: %v", err)
	}
	if snap.App != "Desktop" {
		t.Errorf("got App=%q; want Desktop", snap.App)
	}
}
