package exporter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
	"github.com/tiagovilasboas/antigravity-operator/internal/watcher"
)

type ExportOptions struct {
	Format         string // "markdown" ou "html"
	TargetDir      string // Diretório base do projeto (onde está .agents/session)
	OutputPath     string // Opcional, arquivo de saída
	TranscriptPath string // Caminho para transcript.jsonl se existir
}

type Metrics struct {
	ToolCalls        int
	Thoughts         int
	SubagentsInvoked int
	Duration         string
	Start            time.Time
	End              time.Time
}

type Decision struct {
	Title       string
	Description string
}

type TodoList struct {
	Done    []string
	Pending []string
}

type ReportData struct {
	Date      string
	Summary   *session.Summary
	Decisions []Decision
	Metrics   *Metrics
	TodoList  TodoList
	Progress  int
}

// Export gera um relatório consolidado da sessão.
func Export(opts ExportOptions) (string, error) {
	if opts.Format != "markdown" && opts.Format != "html" {
		return "", fmt.Errorf("formato inválido: %s, use 'markdown' ou 'html'", opts.Format)
	}

	summary, err := session.GetSummary(opts.TargetDir)
	if err != nil {
		return "", fmt.Errorf("falha ao carregar sessão: %w", err)
	}

	decisions := parseDecisions(opts.TargetDir)
	todoList := parseTodo(opts.TargetDir)

	var metrics *Metrics
	if opts.TranscriptPath != "" {
		metrics = parseTranscript(opts.TranscriptPath)
	}

	progress := 0
	if summary.TotalTasks > 0 {
		progress = (summary.DoneTasks * 100) / summary.TotalTasks
	}

	data := ReportData{
		Date:      time.Now().Format("2006-01-02 15:04:05"),
		Summary:   summary,
		Decisions: decisions,
		Metrics:   metrics,
		TodoList:  todoList,
		Progress:  progress,
	}

	var output string
	if opts.Format == "html" {
		output, err = generateHTML(data)
	} else {
		output, err = generateMarkdown(data)
	}

	if err != nil {
		return "", err
	}

	if opts.OutputPath != "" {
		if err := os.WriteFile(opts.OutputPath, []byte(output), 0644); err != nil {
			return "", fmt.Errorf("falha ao salvar relatório: %w", err)
		}
	}

	return output, nil
}

func parseDecisions(targetDir string) []Decision {
	decisionsFile := filepath.Join(targetDir, ".agents", "session", "decisions.md")
	f, err := os.Open(decisionsFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	var decisions []Decision
	var current Decision
	scanner := bufio.NewScanner(f)
	inDecision := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "### ") {
			if inDecision {
				decisions = append(decisions, current)
			}
			inDecision = true
			current = Decision{
				Title: strings.TrimPrefix(line, "### "),
			}
		} else if inDecision {
			if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
				decisions = append(decisions, current)
				inDecision = false
				continue
			}
			if current.Description == "" {
				current.Description = line
			} else {
				current.Description += "\n" + line
			}
		}
	}
	if inDecision {
		decisions = append(decisions, current)
	}

	for i := range decisions {
		decisions[i].Description = strings.TrimSpace(decisions[i].Description)
	}

	return decisions
}

func parseTodo(targetDir string) TodoList {
	todoFile := filepath.Join(targetDir, ".agents", "session", "todo.md")
	f, err := os.Open(todoFile)
	if err != nil {
		return TodoList{}
	}
	defer f.Close()

	var list TodoList
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
			list.Done = append(list.Done, strings.TrimSpace(line[5:]))
		} else if strings.HasPrefix(line, "- [ ]") {
			list.Pending = append(list.Pending, strings.TrimSpace(line[5:]))
		}
	}
	return list
}

func parseTranscript(path string) *Metrics {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	metrics := &Metrics{}
	_ = watcher.ForEachLine(f, watcher.DefaultMaxLineBytes, func(line []byte) error {
		evt, err := watcher.ParseLine(line)
		if err != nil {
			return nil
		}

		if evt.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, evt.CreatedAt)
			if err == nil && !t.IsZero() {
				if metrics.Start.IsZero() || t.Before(metrics.Start) {
					metrics.Start = t
				}
				if t.After(metrics.End) {
					metrics.End = t
				}
			}
		}

		if evt.Type == "PLANNER_RESPONSE" {
			if evt.Thinking != "" {
				metrics.Thoughts++
			}
		}

		for _, tc := range evt.ToolCalls {
			metrics.ToolCalls++
			if tc.Name == "invoke_subagent" {
				metrics.SubagentsInvoked++
			}
		}
		return nil
	})

	if !metrics.Start.IsZero() && !metrics.End.IsZero() {
		d := metrics.End.Sub(metrics.Start)
		metrics.Duration = d.Round(time.Second).String()
	} else {
		metrics.Duration = "0s"
	}

	return metrics
}
