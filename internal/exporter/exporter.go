package exporter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/session"
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
	scanner := bufio.NewScanner(f)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			Type      string `json:"type"`
			CreatedAt string `json:"created_at"`
			Thinking  string `json:"thinking"`
			ToolCalls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		t, _ := time.Parse(time.RFC3339, entry.CreatedAt)
		if !t.IsZero() {
			if metrics.Start.IsZero() {
				metrics.Start = t
			}
			metrics.End = t
		}

		if entry.Type == "PLANNER_RESPONSE" {
			metrics.Thoughts++
		}

		for _, tc := range entry.ToolCalls {
			metrics.ToolCalls++
			if tc.Function.Name == "invoke_subagent" {
				metrics.SubagentsInvoked++
			}
		}
	}

	if !metrics.Start.IsZero() && !metrics.End.IsZero() {
		d := metrics.End.Sub(metrics.Start)
		metrics.Duration = d.Round(time.Second).String()
	} else {
		metrics.Duration = "0s"
	}

	return metrics
}

func generateHTML(data ReportData) (string, error) {
	tmpl, err := template.New("html").Parse(htmlTemplateStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func generateMarkdown(data ReportData) (string, error) {
	funcMap := texttemplate.FuncMap{
		"progressBar": func(p int) string {
			filled := p / 5
			empty := 20 - filled
			return strings.Repeat("█", filled) + strings.Repeat("░", empty)
		},
		"singleLine": func(s string) string {
			s = strings.ReplaceAll(s, "\n", " <br> ")
			s = strings.ReplaceAll(s, "\r", "")
			return s
		},
	}
	tmpl, err := texttemplate.New("md").Funcs(funcMap).Parse(mdTemplateStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const htmlTemplateStr = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Relatório de Sessão</title>
    <style>
        :root {
            --bg-color: #f7f9fc;
            --text-color: #1a202c;
            --card-bg: #ffffff;
            --border-color: #e2e8f0;
            --accent-color: #3182ce;
            --success-color: #38a169;
            --pending-color: #dd6b20;
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg-color: #1a202c;
                --text-color: #e2e8f0;
                --card-bg: #2d3748;
                --border-color: #4a5568;
                --accent-color: #63b3ed;
                --success-color: #68d391;
                --pending-color: #fbd38d;
            }
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
            margin: 0;
            padding: 2rem;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
        }
        .card {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05);
        }
        h1, h2, h3 {
            margin-top: 0;
        }
        .header-meta {
            color: var(--accent-color);
            font-size: 0.9rem;
            margin-bottom: 1rem;
        }
        .progress-bar {
            width: 100%;
            background-color: var(--border-color);
            border-radius: 4px;
            overflow: hidden;
            height: 20px;
            margin-top: 0.5rem;
        }
        .progress-fill {
            height: 100%;
            background-color: var(--success-color);
            width: {{ .Progress }}%;
            transition: width 0.3s ease;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 1rem;
        }
        .metric-box {
            background-color: var(--bg-color);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 1rem;
            text-align: center;
        }
        .metric-value {
            font-size: 1.5rem;
            font-weight: bold;
            color: var(--accent-color);
        }
        .metric-label {
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        th, td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid var(--border-color);
        }
        th {
            background-color: var(--bg-color);
        }
        .task-list {
            list-style: none;
            padding: 0;
        }
        .task-list li {
            padding: 0.5rem 0;
            border-bottom: 1px solid var(--border-color);
            display: flex;
            align-items: center;
        }
        .task-list li:last-child {
            border-bottom: none;
        }
        .task-done::before {
            content: "✅";
            margin-right: 0.5rem;
        }
        .task-pending::before {
            content: "⏳";
            margin-right: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="card">
            <h1>Relatório de Sessão</h1>
            <div class="header-meta">Gerado em: {{ .Date }}</div>
            <p><strong>Objetivo:</strong> {{ .Summary.Objective }}</p>
            <p><strong>Status Final:</strong> {{ .Summary.Status }}</p>
            
            <h3>Progresso: {{ .Progress }}% ({{ .Summary.DoneTasks }}/{{ .Summary.TotalTasks }})</h3>
            <div class="progress-bar">
                <div class="progress-fill"></div>
            </div>
        </div>

        {{ if .Metrics }}
        <div class="card">
            <h2>Métricas de Execução</h2>
            <div class="metrics-grid">
                <div class="metric-box">
                    <div class="metric-value">{{ .Metrics.Duration }}</div>
                    <div class="metric-label">Duração</div>
                </div>
                <div class="metric-box">
                    <div class="metric-value">{{ .Metrics.ToolCalls }}</div>
                    <div class="metric-label">Tool Calls</div>
                </div>
                <div class="metric-box">
                    <div class="metric-value">{{ .Metrics.Thoughts }}</div>
                    <div class="metric-label">Pensamentos</div>
                </div>
                <div class="metric-box">
                    <div class="metric-value">{{ .Metrics.SubagentsInvoked }}</div>
                    <div class="metric-label">Subagentes</div>
                </div>
            </div>
        </div>
        {{ end }}

        <div class="card">
            <h2>Tarefas</h2>
            <ul class="task-list">
                {{ range .TodoList.Done }}
                <li class="task-done">{{ . }}</li>
                {{ end }}
                {{ range .TodoList.Pending }}
                <li class="task-pending">{{ . }}</li>
                {{ end }}
            </ul>
        </div>

        <div class="card">
            <h2>Decisões de Arquitetura</h2>
            {{ if .Decisions }}
            <table>
                <thead>
                    <tr>
                        <th>Decisão</th>
                        <th>Contexto e Trade-offs</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range .Decisions }}
                    <tr>
                        <td><strong>{{ .Title }}</strong></td>
                        <td><pre style="white-space: pre-wrap; font-family: inherit; margin:0;">{{ .Description }}</pre></td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
            {{ else }}
            <p>Nenhuma decisão de arquitetura documentada nesta sessão.</p>
            {{ end }}
        </div>
    </div>
</body>
</html>`

const mdTemplateStr = `# Relatório de Sessão

**Gerado em:** {{ .Date }}  
**Objetivo:** {{ .Summary.Objective }}  
**Status Final:** {{ .Summary.Status }}  

## Progresso ({{ .Progress }}%)
**Concluídas:** {{ .Summary.DoneTasks }} | **Total:** {{ .Summary.TotalTasks }}

Progresso: [{{ progressBar .Progress }}] {{ .Progress }}%

{{ if .Metrics }}
## Métricas de Execução
- **Duração:** {{ .Metrics.Duration }}
- **Tool Calls:** {{ .Metrics.ToolCalls }}
- **Pensamentos (Thoughts):** {{ .Metrics.Thoughts }}
- **Subagentes Invocados:** {{ .Metrics.SubagentsInvoked }}
{{ end }}

## Tarefas

### Concluídas ✅
{{ range .TodoList.Done }}- [x] {{ . }}
{{ end }}{{ if not .TodoList.Done }}- Nenhuma tarefa concluída.
{{ end }}
### Pendentes ⏳
{{ range .TodoList.Pending }}- [ ] {{ . }}
{{ end }}{{ if not .TodoList.Pending }}- Nenhuma tarefa pendente.
{{ end }}
## Decisões de Arquitetura e Trade-offs
{{ if .Decisions }}
| Decisão | Contexto e Trade-offs |
|---------|-----------------------|
{{ range .Decisions }}| **{{ .Title }}** | {{ singleLine .Description }} |
{{ end }}{{ else }}
*Nenhuma decisão documentada.*
{{ end }}
`
