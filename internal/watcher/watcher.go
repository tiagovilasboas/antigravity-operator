package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ToolCall representa uma ferramenta invocada pelo modelo.
type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Event representa um registro estruturado da transcrição (transcript.jsonl).
type Event struct {
	StepIndex  int        `json:"step_index"`
	Source     string     `json:"source"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	CreatedAt  string     `json:"created_at"`
	Content    string     `json:"content"`
	Thinking   string     `json:"thinking"`
	ToolCalls  []ToolCall `json:"tool_calls"`
	IsQuestion bool       `json:"-"`
	IsDone     bool       `json:"-"`
}

// TranscriptInfo contém metadados de uma sessão descoberta no disco.
type TranscriptInfo struct {
	Path           string
	ConversationID string
	ModTime        time.Time
	Size           int64
}

// AppDataDirs lists the Antigravity app data directories under home whose
// brain/<conversation-id>/.system_generated/logs/transcript.jsonl hold session
// transcripts: ~/.gemini/antigravity (Antigravity 2.0), ~/.gemini/antigravity-cli
// (CLI) and ~/.gemini/antigravity-ide (IDE), per the transcriptPath field in
// https://antigravity.google/docs/hooks/. It is the single source of truth for
// where agyo looks for transcripts.
func AppDataDirs(home string) []string {
	return []string{
		filepath.Join(home, ".gemini", "antigravity"),
		filepath.Join(home, ".gemini", "antigravity-cli"),
		filepath.Join(home, ".gemini", "antigravity-ide"),
	}
}

// FindLatestTranscript returns the most recently modified transcript across
// every directory in AppDataDirs(home).
func FindLatestTranscript(home string) (*TranscriptInfo, error) {
	var latest *TranscriptInfo
	dirs := AppDataDirs(home)
	for _, dir := range dirs {
		t, err := FindActiveTranscript(dir)
		if err != nil {
			continue
		}
		if latest == nil || t.ModTime.After(latest.ModTime) {
			latest = t
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("nenhum transcript ativo encontrado em %s", strings.Join(dirs, ", "))
	}
	return latest, nil
}

// FindActiveTranscript localiza a sessão mais recente em <appDataDir>/brain/*/transcript.jsonl.
// Prefer FindLatestTranscript, which searches every Antigravity app data dir.
func FindActiveTranscript(geminiDir string) (*TranscriptInfo, error) {
	brainDir := filepath.Join(geminiDir, "brain")
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler pasta brain: %w", err)
	}

	var latest *TranscriptInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(brainDir, entry.Name(), ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(candidate)
		if err == nil {
			if latest == nil || info.ModTime().After(latest.ModTime) {
				latest = &TranscriptInfo{
					Path:           candidate,
					ConversationID: entry.Name(),
					ModTime:        info.ModTime(),
					Size:           info.Size(),
				}
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("nenhum transcript ativo encontrado em %s", brainDir)
	}

	return latest, nil
}

// ParseLine decodifica uma linha JSONL em um Event estruturado.
func ParseLine(line []byte) (*Event, error) {
	clean := strings.TrimSpace(string(line))
	if len(clean) == 0 {
		return nil, fmt.Errorf("linha vazia")
	}

	var evt Event
	if err := json.Unmarshal([]byte(clean), &evt); err != nil {
		return nil, err
	}

	for _, call := range evt.ToolCalls {
		if call.Name == "ask_question" {
			evt.IsQuestion = true
		}
	}
	if evt.Status == "DONE" {
		evt.IsDone = true
	}

	return &evt, nil
}

// DefaultMaxLineBytes define o limite maximo padrao por linha (10 MB).
const DefaultMaxLineBytes = 10 * 1024 * 1024

// ForEachLine le linhas de r de forma resiliente ate maxBytes por linha (padrao 10MB se <= 0).
// Linhas que excederem o limite sao ignoradas com seguranca.
func ForEachLine(r io.Reader, maxBytes int, fn func(line []byte) error) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxLineBytes
	}
	reader := bufio.NewReaderSize(r, 64*1024)

	for {
		var line []byte
		var oversized bool
		for {
			chunk, isPrefix, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF {
					if len(chunk) > 0 && !oversized {
						if len(line)+len(chunk) > maxBytes {
							oversized = true
							line = nil
						} else {
							line = append(line, chunk...)
						}
					}
					break
				}
				return err
			}
			if !oversized {
				if len(line)+len(chunk) > maxBytes {
					oversized = true
					line = nil
				} else {
					line = append(line, chunk...)
				}
			}
			if !isPrefix {
				break
			}
		}
		if oversized || len(line) == 0 {
			if _, err := reader.Peek(1); err != nil {
				break
			}
			continue
		}

		if err := fn(line); err != nil {
			return err
		}
	}
	return nil
}

// NotifyExecutor permite interceptar comandos de notificação no SO (útil para testes ou mocks).
var NotifyExecutor = func(osName, title, message string) {
	switch osName {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "linux":
		_ = exec.Command("notify-send", title, message).Run()
	}
}

// Notify emite notificação visual no SO (macOS/Linux) e alerta sonoro no terminal.
func Notify(osName, title, message string) {
	if os.Getenv("AGYO_DISABLE_NOTIFY") == "1" {
		return
	}
	// Alerta sonoro de terminal (bell)
	fmt.Print("\a")

	NotifyExecutor(osName, title, message)
}

// WatchOptions configura o comportamento do tailing.
type WatchOptions struct {
	Follow       bool
	NotifyOnWait bool
	PollInterval time.Duration
	InitialSteps int
	OSName       string
}

// Stream acompanha o arquivo transcript.jsonl e envia eventos para o handler.
func Stream(ctx context.Context, transcriptPath string, opts WatchOptions, handler func(*Event)) error {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return fmt.Errorf("falha ao abrir transcript: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var window []*Event
	if opts.InitialSteps > 0 {
		window = make([]*Event, 0, opts.InitialSteps)
	}
	var allEvents []*Event
	var partial []byte

	// 1. Fase Inicial: lê tudo o que já existe
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if len(partial) > 0 {
				line = append(partial, line...)
				partial = nil
			}
			if line[len(line)-1] == '\n' {
				if evt, pErr := ParseLine(line); pErr == nil {
					globalSubagentTree.ProcessEvent(evt)
					if opts.InitialSteps > 0 {
						if len(window) < opts.InitialSteps {
							window = append(window, evt)
						} else {
							copy(window, window[1:])
							window[len(window)-1] = evt
						}
					} else {
						allEvents = append(allEvents, evt)
					}
				}
			} else {
				partial = append(partial, line...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Emite os passos iniciais solicitados (ex: últimos N passos)
	if opts.InitialSteps > 0 {
		for _, evt := range window {
			handler(evt)
		}
	} else {
		for _, evt := range allEvents {
			handler(evt)
		}
	}

	if !opts.Follow {
		return nil
	}

	poll := opts.PollInterval
	if poll <= 0 {
		poll = 400 * time.Millisecond
	}

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	// 2. Fase de Tailing: escuta novos dados continuamente
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for {
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					if len(partial) > 0 {
						line = append(partial, line...)
						partial = nil
					}
					if line[len(line)-1] == '\n' {
						if evt, pErr := ParseLine(line); pErr == nil {
							globalSubagentTree.ProcessEvent(evt)
							handler(evt)
							if opts.NotifyOnWait && evt.IsQuestion {
								Notify(opts.OSName, "Antigravity Operator", "O agente precisa da sua resposta (pergunta interativa)!")
							}
						}
					} else {
						partial = append(partial, line...)
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
		}
	}
}
