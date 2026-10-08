package platform_test

import (
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestDetect(t *testing.T) {
	info, err := platform.Detect()
	if err != nil {
		t.Fatalf("esperava sucesso na detecção da plataforma, obteve erro: %v", err)
	}

	if info.OS == "" {
		t.Errorf("esperava OS preenchido")
	}

	if info.HomeDir == "" {
		t.Errorf("esperava HomeDir preenchido")
	}

	if info.BrowserProfile == "" {
		t.Errorf("esperava BrowserProfile preenchido")
	}
}
