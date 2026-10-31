package computer

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type mockEngine struct {
	inspectCalled bool
	executeCalled bool
	captureCalled bool
}

func (m *mockEngine) Inspect(ctx context.Context) (*Snapshot, error) {
	m.inspectCalled = true
	return &Snapshot{
		App: "MockApp",
		Elements: []*Element{
			{Ref: "ref_1", Role: "AXButton", Title: "Submit"},
		},
	}, nil
}

func (m *mockEngine) Execute(ctx context.Context, action Action) error {
	m.executeCalled = true
	return nil
}

func (m *mockEngine) ExecuteBatch(ctx context.Context, actions []Action) error {
	m.executeCalled = true
	return nil
}

func (m *mockEngine) Capture(ctx context.Context, opts ShotOptions) ([]byte, string, error) {
	m.captureCalled = true
	return []byte("FAKE_IMAGE_BYTES"), "/tmp/fake.jpg", nil
}

func TestServeMCP_InitializeAndList(t *testing.T) {
	mock := &mockEngine{}

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var out bytes.Buffer
	err := ServeMCP(mock, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d responses; want 2 (output: %q)", len(lines), out.String())
	}

	var initResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("unmarshal init error: %v", err)
	}
	if initResp.Error != nil {
		t.Errorf("init returned error: %v", initResp.Error)
	}

	var listResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("unmarshal list error: %v", err)
	}
	resMap, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("list result is not map: %v", listResp.Result)
	}
	tools, ok := resMap["tools"].([]interface{})
	if !ok || len(tools) != 4 {
		t.Fatalf("tools count = %d; want 4", len(tools))
	}
}

func TestServeMCP_ToolsCall(t *testing.T) {
	mock := &mockEngine{}

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"computer_inspect","arguments":{}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"computer_action","arguments":{"action":"click","ref":"ref_1"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"computer_shot","arguments":{"quality":80}}}
`
	var out bytes.Buffer
	err := ServeMCP(mock, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d responses; want 3", len(lines))
	}

	if !mock.inspectCalled {
		t.Errorf("Inspect was not called on mock engine")
	}
	if !mock.executeCalled {
		t.Errorf("Execute was not called on mock engine")
	}
	if !mock.captureCalled {
		t.Errorf("Capture was not called on mock engine")
	}
}
