package computer

import (
	"testing"
)

func TestRect_Center(t *testing.T) {
	r := Rect{X: 100, Y: 200, Width: 50, Height: 80}
	cx, cy := r.Center()
	if cx != 125 || cy != 240 {
		t.Errorf("Center() = (%d, %d); want (125, 240)", cx, cy)
	}
}

func TestSnapshot_FindElementByRef(t *testing.T) {
	snap := &Snapshot{
		App: "TestApp",
		Elements: []*Element{
			{
				Ref:   "ref_1",
				Role:  "AXWindow",
				Title: "Main",
				Children: []*Element{
					{
						Ref:   "ref_2",
						Role:  "AXButton",
						Title: "OK",
						Bounds: Rect{
							X:      10,
							Y:      20,
							Width:  80,
							Height: 30,
						},
					},
				},
			},
		},
	}

	el := snap.FindElementByRef("ref_2")
	if el == nil {
		t.Fatalf("FindElementByRef(ref_2) returned nil")
	}
	if el.Title != "OK" {
		t.Errorf("el.Title = %q; want %q", el.Title, "OK")
	}

	if missing := snap.FindElementByRef("ref_99"); missing != nil {
		t.Errorf("FindElementByRef(ref_99) = %v; want nil", missing)
	}
}

func TestValidateAction(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr bool
	}{
		{
			name:    "valid click by coords",
			action:  Action{Type: ActionClick, X: 100, Y: 100},
			wantErr: false,
		},
		{
			name:    "valid click by ref",
			action:  Action{Type: ActionClick, Ref: "ref_1"},
			wantErr: false,
		},
		{
			name:    "invalid click missing coords and ref",
			action:  Action{Type: ActionClick, X: -1, Y: -1},
			wantErr: true,
		},
		{
			name:    "valid type",
			action:  Action{Type: ActionTypeString, Text: "hello world"},
			wantErr: false,
		},
		{
			name:    "invalid type empty text",
			action:  Action{Type: ActionTypeString},
			wantErr: true,
		},
		{
			name:    "valid key",
			action:  Action{Type: ActionKey, Key: "return"},
			wantErr: false,
		},
		{
			name:    "valid drag",
			action:  Action{Type: ActionDrag, X: 10, Y: 10, EndX: 50, EndY: 50},
			wantErr: false,
		},
		{
			name:    "invalid action type",
			action:  Action{Type: "invalid_action"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAction(tc.action)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateAction() error = %v; wantErr %v", err, tc.wantErr)
			}
		})
	}
}
