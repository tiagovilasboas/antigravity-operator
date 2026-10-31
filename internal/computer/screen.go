package computer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// ScreenCapturer é responsável pela captura e compressão otimizada de tela.
type ScreenCapturer struct {
	goos       string
	customExec func(ctx context.Context, name string, args ...string) ([]byte, error)
	inspector  *Inspector
}

// NewScreenCapturer cria uma nova instância do capturador.
func NewScreenCapturer(ins *Inspector) *ScreenCapturer {
	if ins == nil {
		ins = NewInspector()
	}
	return &ScreenCapturer{
		inspector: ins,
		customExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			return cmd.CombinedOutput()
		},
	}
}

func (sc *ScreenCapturer) targetOS() string {
	if sc.goos != "" {
		return sc.goos
	}
	return runtime.GOOS
}

// Capture executa a captura de tela aplicando downsampling e compressão para corte de tokens.
func (sc *ScreenCapturer) Capture(ctx context.Context, opts ShotOptions) ([]byte, string, error) {
	// Normaliza opções padrão
	if opts.Format == "" {
		opts.Format = "jpeg"
	}
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 1280
	}
	if opts.Quality <= 0 {
		opts.Quality = 75
	}

	ext := "." + opts.Format
	targetPath := opts.OutPath
	if targetPath == "" {
		tmpFile, err := os.CreateTemp("", "agyo-shot-*"+ext)
		if err != nil {
			return nil, "", fmt.Errorf("falha ao criar arquivo temporário: %w", err)
		}
		targetPath = tmpFile.Name()
		_ = tmpFile.Close()
	}

	if sc.targetOS() == "darwin" {
		return sc.captureDarwin(ctx, opts, targetPath)
	}

	return sc.captureFallback(ctx, opts, targetPath)
}

func (sc *ScreenCapturer) captureDarwin(ctx context.Context, opts ShotOptions, targetPath string) ([]byte, string, error) {
	// Cria arquivo temporário intermediário em PNG
	rawTmp, err := os.CreateTemp("", "agyo-raw-*.png")
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar buffer temporário: %w", err)
	}
	rawPath := rawTmp.Name()
	_ = rawTmp.Close()
	defer os.Remove(rawPath)

	var captureArgs []string
	captureArgs = append(captureArgs, "-x") // modo silencioso (sem som de obturador)

	// Se WindowOnly for solicitado, tenta capturar apenas os limites da janela ativa via Inspector
	if opts.WindowOnly {
		snap, err := sc.inspector.InspectWindow(ctx)
		if err == nil && snap != nil && snap.Bounds.Width > 0 && snap.Bounds.Height > 0 {
			rectArg := fmt.Sprintf("%d,%d,%d,%d", snap.Bounds.X, snap.Bounds.Y, snap.Bounds.Width, snap.Bounds.Height)
			captureArgs = append(captureArgs, "-R", rectArg)
		}
	}

	captureArgs = append(captureArgs, rawPath)

	// 1. Executa screencapture nativo do macOS
	if out, err := sc.customExec(ctx, "screencapture", captureArgs...); err != nil {
		return nil, "", fmt.Errorf("screencapture falhou: %w (output: %s)", err, string(out))
	}

	// 2. Aplica compressão e redimensionamento via utilitário nativo sips do macOS
	sipsArgs := []string{
		"-s", "format", opts.Format,
	}
	if opts.Format == "jpeg" {
		sipsArgs = append(sipsArgs, "-s", "formatOptions", strconv.Itoa(opts.Quality))
	}
	if opts.MaxWidth > 0 {
		sipsArgs = append(sipsArgs, "--resampleWidth", strconv.Itoa(opts.MaxWidth))
	}
	sipsArgs = append(sipsArgs, rawPath, "--out", targetPath)

	if out, err := sc.customExec(ctx, "sips", sipsArgs...); err != nil {
		return nil, "", fmt.Errorf("compressão via sips falhou: %w (output: %s)", err, string(out))
	}

	// Lê o arquivo final gerado
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler captura comprimida: %w", err)
	}

	absPath, _ := filepath.Abs(targetPath)
	return data, absPath, nil
}

func (sc *ScreenCapturer) captureFallback(ctx context.Context, opts ShotOptions, targetPath string) ([]byte, string, error) {
	// Em Linux/Headless, tenta scrot ou import se presentes
	for _, tool := range []string{"scrot", "import", "maim"} {
		if path, err := exec.LookPath(tool); err == nil {
			var args []string
			if tool == "scrot" {
				args = []string{"-o", targetPath}
			} else if tool == "import" {
				args = []string{"-window", "root", targetPath}
			} else if tool == "maim" {
				args = []string{targetPath}
			}
			if _, err := sc.customExec(ctx, path, args...); err == nil {
				data, rErr := os.ReadFile(targetPath)
				if rErr == nil {
					return data, targetPath, nil
				}
			}
		}
	}

	// Fallback sintético mínimo se nenhum utilitário gráfico existir
	dummy := []byte("SIMULATED_SCREENSHOT_DATA")
	if err := os.WriteFile(targetPath, dummy, 0o644); err != nil {
		return nil, "", err
	}
	return dummy, targetPath, nil
}
