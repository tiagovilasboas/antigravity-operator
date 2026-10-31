package computer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenCapturer_CaptureDarwin_Mock(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "screenshot.jpg")

	var executedCommands []string
	sc := &ScreenCapturer{
		goos: "darwin",
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executedCommands = append(executedCommands, name+" "+strings.Join(args, " "))
			// Se for sips gerando outPath, cria arquivo simulado
			for i, arg := range args {
				if arg == "--out" && i+1 < len(args) {
					_ = os.WriteFile(args[i+1], []byte("MOCK_JPEG_CONTENT"), 0o644)
				}
			}
			return []byte("ok"), nil
		},
		inspector: &Inspector{
			goos: "darwin",
			customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(`{"app":"App","bounds":{"x":10,"y":20,"width":800,"height":600},"elements":[]}`), nil
			},
		},
	}

	opts := ShotOptions{
		WindowOnly: true,
		Format:     "jpeg",
		MaxWidth:   1024,
		Quality:    80,
		OutPath:    outPath,
	}

	data, savedPath, err := sc.Capture(context.Background(), opts)
	if err != nil {
		t.Fatalf("captureDarwin() error = %v", err)
	}

	if string(data) != "MOCK_JPEG_CONTENT" {
		t.Errorf("got data %q; want MOCK_JPEG_CONTENT", string(data))
	}
	if savedPath != outPath {
		t.Errorf("got savedPath %q; want %q", savedPath, outPath)
	}

	// Verifica se screencapture foi chamado com -R 10,20,800,600
	foundRect := false
	for _, cmd := range executedCommands {
		if strings.Contains(cmd, "-R 10,20,800,600") {
			foundRect = true
			break
		}
	}
	if !foundRect {
		t.Errorf("screencapture was not called with -R bounds in commands: %v", executedCommands)
	}
}

func TestScreenCapturer_CaptureFallback(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "fallback.jpg")

	sc := NewScreenCapturer(nil)
	data, _, err := sc.captureFallback(context.Background(), ShotOptions{}, outPath)
	if err != nil {
		t.Fatalf("captureFallback error = %v", err)
	}
	if len(data) == 0 {
		t.Errorf("expected non-empty data from fallback")
	}
}
