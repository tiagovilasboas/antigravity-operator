package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
	"github.com/tiagovilasboas/antigravity-operator/templates"
)

var studyStdin io.Reader = os.Stdin

const maxStudyPayloadBytes = 32 * 1024 // 32KB cap para FinOps e prevencao de context bloat

func printStudyUsage() {
	fmt.Println(`agyo study - Academic Study Assistant & Student Workflow
Synthesize video lectures, schedule exam preparation, and practice interactive quizzes.

Usage:
  agyo study <subcommand> [arguments]

Available subcommands:
  ingest <url> --notebook <id> [--out <dir>] [--title <title>]
      Absorb YouTube lecture or article into NotebookLM and generate Feynman study guide and Anki flashcards
  exam-prep --disciplina <nome> --data <YYYY-MM-DD> [--dias <N>]
      Calculate reverse study schedule, register in .agents/session/todo.md, and sync with Google Calendar
  quiz --notebook <id> [--topic <nome>]
      Interactive Feynman checkpoint: generate a challenging question, evaluate your answer with instant feedback

Use 'agyo study <subcommand> --help' for details on each subcommand.`)
}

func runStudy(info *platform.Info, args []string) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printStudyUsage()
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "ingest":
		runStudyIngest(info, subArgs)
	case "exam-prep", "examprep", "prep":
		runStudyExamPrep(info, subArgs)
	case "quiz", "feynman":
		runStudyQuiz(info, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Subcomando study desconhecido: %q\n", subcommand)
		printStudyUsage()
		exitFunc(ExitUsageError)
	}
}

func runStudyIngest(info *platform.Info, args []string) {
	fs := flag.NewFlagSet("study ingest", flag.ContinueOnError)
	notebookID := fs.String("notebook", "", "ID do caderno do Google NotebookLM")
	outDir := fs.String("out", "", "Pasta de destino para o artigo e flashcards (padrao: estudos/<tema>)")
	title := fs.String("title", "", "Titulo opcional do conteudo ingerido")
	port := fs.Int("port", 0, "Porta DevTools do Chrome")

	if err := fs.Parse(permuteArgs(args, nil)); err != nil {
		return
	}

	positional := fs.Args()
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "Erro: URL ou texto da fonte e obrigatorio.")
		fmt.Fprintln(os.Stderr, "Uso: agyo study ingest <url> --notebook <id> [--out <dir>]")
		exitFunc(ExitUsageError)
		return
	}

	sourceTarget := strings.TrimSpace(positional[0])
	if *notebookID == "" {
		fmt.Fprintln(os.Stderr, "Erro: flag --notebook <id> e obrigatoria.")
		exitFunc(ExitUsageError)
		return
	}

	fmt.Printf("🔍 Ingerindo fonte no caderno %s...\n", *notebookID)

	// 1. Adiciona a fonte (URL ou nota direta)
	isURL := strings.HasPrefix(sourceTarget, "http://") || strings.HasPrefix(sourceTarget, "https://")
	if isURL {
		if err := notebook.AddSourceURL(info, *port, *notebookID, sourceTarget); err != nil {
			fmt.Fprintf(os.Stderr, "Aviso ao adicionar URL: %v (tentando prosseguir com analise)\n", err)
		} else {
			fmt.Println("✅ Fonte adicionada ao NotebookLM com sucesso.")
		}
	} else {
		noteTitle := *title
		if noteTitle == "" {
			noteTitle = "Nota de Estudo"
		}
		if err := notebook.PushSource(info, *port, *notebookID, noteTitle, sourceTarget); err != nil {
			fmt.Fprintf(os.Stderr, "Aviso ao injetar nota: %v\n", err)
		} else {
			fmt.Println("✅ Nota injetada no NotebookLM com sucesso.")
		}
	}

	// 2. Consulta fundamentada via NotebookLM para sintese Feynman
	fmt.Println("🧠 Gerando sintese conceitual e flashcards via NotebookLM...")
	query := "Gere um guia de estudo didatico no metodo Feynman com: 1. Resumo conceitual dos pontos centrais; 2. Termos-chave e definicoes; 3. 5 perguntas de verificacao com respostas explicativas no formato Pergunta / Resposta."
	res, err := notebook.AskNotebook(info, *port, *notebookID, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao consultar NotebookLM: %v\n", err)
		exitFunc(ExitRuntimeError)
		return
	}

	answerText := res.Answer
	if len(answerText) > maxStudyPayloadBytes {
		cut := maxStudyPayloadBytes
		for cut > 0 && !utf8.RuneStart(answerText[cut]) {
			cut--
		}
		answerText = answerText[:cut]
	}

	// 3. Determina pasta de destino com prevencao contra path traversal (SEC-STU-01)
	destFolder := *outDir
	if destFolder != "" {
		destFolder = filepath.Clean(destFolder)
		if filepath.IsAbs(destFolder) || strings.HasPrefix(destFolder, "..") || strings.Contains(destFolder, "/../") {
			destFolder = filepath.Join("estudos", filepath.Base(destFolder))
		}
	} else {
		topicSlug := "geral"
		if *title != "" {
			topicSlug = slugify(*title)
		} else if isURL {
			topicSlug = "fonte-" + time.Now().Format("20060102-1504")
		}
		destFolder = filepath.Join("estudos", topicSlug)
	}

	if err := os.MkdirAll(destFolder, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao criar pasta %s: %v\n", destFolder, err)
		exitFunc(ExitRuntimeError)
		return
	}

	// 4. Salva artigo de estudo
	articlePath := filepath.Join(destFolder, "artigo.md")
	header := fmt.Sprintf("# Guia de Estudo: %s\n\n- Data: %s\n- Fonte: %s\n- Caderno NotebookLM: %s\n\n---\n\n",
		ifEmpty(*title, "Sintese de Conteudo"),
		time.Now().Format("2006-01-02 15:04"),
		sourceTarget,
		*notebookID,
	)

	templateBase, _ := templates.FS.ReadFile("skills/video-to-article/templates/estudo-template.md")
	var fullArticle strings.Builder
	fullArticle.WriteString(header)
	if len(templateBase) > 0 {
		fullArticle.WriteString(string(templateBase))
		fullArticle.WriteString("\n\n---\n\n## Conteudo Sintetizado pelo NotebookLM\n\n")
	}
	fullArticle.WriteString(answerText)
	if len(res.Citations) > 0 {
		fullArticle.WriteString("\n\n### Citacoes e Fontes Fundamentadas\n")
		for _, c := range res.Citations {
			fullArticle.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	if err := os.WriteFile(articlePath, []byte(fullArticle.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao gravar artigo em %s: %v\n", articlePath, err)
		exitFunc(ExitRuntimeError)
		return
	}
	fmt.Printf("📄 Artigo de estudo salvo em: %s\n", articlePath)

	// 5. Gera flashcards no formato Anki / Obsidian
	flashcards := generateFlashcards(answerText)
	flashcardsPath := filepath.Join(destFolder, "flashcards.md")
	if err := os.WriteFile(flashcardsPath, []byte(flashcards), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao gravar flashcards em %s: %v\n", flashcardsPath, err)
	} else {
		fmt.Printf("🗂️  Flashcards Anki/Obsidian salvos em: %s\n", flashcardsPath)
	}
}

func runStudyExamPrep(info *platform.Info, args []string) {
	fs := flag.NewFlagSet("study exam-prep", flag.ContinueOnError)
	disciplina := fs.String("disciplina", "", "Nome da disciplina ou prova")
	dataStr := fs.String("data", "", "Data da prova no formato YYYY-MM-DD, DD/MM/YYYY ou expressao relativa")
	dias := fs.Int("dias", 7, "Quantidade de dias de preparacao reversa (padrao: 7)")
	contextFlag := fs.String("context", "pessoal", "Contexto de autenticacao Google Workspace (padrao: pessoal)")
	icsPath := fs.String("ics", "", "Exportar cronograma de preparacao para arquivo iCalendar (.ics)")
	webFlag := fs.Bool("web", false, "Gerar URL web do Google Calendar para abertura direta no navegador")

	if err := fs.Parse(args); err != nil {
		return
	}

	cleanDisciplina := strings.Join(strings.Fields(*disciplina), " ")

	if cleanDisciplina == "" || *dataStr == "" {
		fmt.Fprintln(os.Stderr, "Erro: as flags --disciplina <nome> e --data <YYYY-MM-DD> sao obrigatorias.")
		fmt.Fprintln(os.Stderr, "Uso: agyo study exam-prep --disciplina \"Calculo III\" --data 2026-11-20 [--dias 7] [--context pessoal] [--ics cronograma.ics] [--web]")
		exitFunc(ExitUsageError)
		return
	}

	examDate, err := workspace.ParseFlexTime(*dataStr, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Formato de data invalido: %v (use YYYY-MM-DD, DD/MM/YYYY ou expressoes relativas)\n", err)
		exitFunc(ExitUsageError)
		return
	}

	fmt.Printf("🎯 Plano de Preparacao Reversa para: %s (Data da Prova: %s)\n", cleanDisciplina, examDate.Format("2006-01-02"))

	// Define marcos do cronograma reverso
	type milestone struct {
		DaysBefore int
		Title      string
		Desc       string
	}

	plan := []milestone{
		{DaysBefore: *dias, Title: fmt.Sprintf("Fundamentos e Leitura Base de %s", cleanDisciplina), Desc: "Revisao dos conceitos centrais e resumos"},
		{DaysBefore: *dias * 5 / 7, Title: fmt.Sprintf("Resolucao de Exercicios de %s", cleanDisciplina), Desc: "Pratica intensiva de listas e exercicios chave"},
		{DaysBefore: *dias * 3 / 7, Title: fmt.Sprintf("Simulado e Mapeamento de Gaps de %s", cleanDisciplina), Desc: "Simulado contra o relogio e levantamento de duvidas"},
		{DaysBefore: 1, Title: fmt.Sprintf("Revisao Leve e Flashcards de %s", cleanDisciplina), Desc: "Passada rapida nos flashcards e descanso mental"},
		{DaysBefore: 0, Title: fmt.Sprintf("Avaliacao: Dia da Prova de %s", cleanDisciplina), Desc: "Dia do exame presencial ou online"},
	}

	// 1. Registra tarefas em .agents/session/todo.md se existir (SEC-STU-03)
	todoPath := filepath.Join(".agents", "session", "todo.md")
	var addedTasks []string
	if todoBytes, err := os.ReadFile(todoPath); err == nil {
		content := string(todoBytes)
		for _, m := range plan {
			taskDate := examDate.AddDate(0, 0, -m.DaysBefore).Format("2006-01-02")
			taskLine := fmt.Sprintf("- [ ] %s: %s (%s)", taskDate, m.Title, m.Desc)
			if !strings.Contains(content, m.Title) {
				addedTasks = append(addedTasks, taskLine)
			}
		}
		if len(addedTasks) > 0 {
			updated := content + "\n\n### Cronograma de Prova: " + cleanDisciplina + "\n" + strings.Join(addedTasks, "\n") + "\n"
			_ = os.WriteFile(todoPath, []byte(updated), 0644)
			fmt.Printf("📝 %d tarefas adicionadas a %s\n", len(addedTasks), todoPath)
		}
	}

	// 2. Monta eventos de calendario para sincronizacao e exportacao
	var calEvents []workspace.CalendarEvent
	for _, m := range plan {
		targetDay := examDate.AddDate(0, 0, -m.DaysBefore)
		startTime := time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 19, 0, 0, 0, time.Local)
		endTime := startTime.Add(2 * time.Hour)
		if m.DaysBefore == 0 {
			startTime = time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 9, 0, 0, 0, time.Local)
			endTime = startTime.Add(3 * time.Hour)
		}
		calEvents = append(calEvents, workspace.CalendarEvent{
			Summary:     m.Title,
			Description: fmt.Sprintf("Bloco de preparacao academica: %s", m.Desc),
			Start:       startTime,
			End:         endTime,
		})
	}

	// 3. Exporta arquivo iCalendar (.ics) se solicitado (PLUG-03)
	if *icsPath != "" {
		cwd, _ := os.Getwd()
		baseDir := cwd
		if filepath.IsAbs(*icsPath) {
			baseDir = filepath.Dir(*icsPath)
		}
		icsData := workspace.CalendarEventsToICS(calEvents)
		if err := workspace.WriteSecureICSFile(*icsPath, []byte(icsData), baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "Aviso ao exportar arquivo .ics: %v\n", err)
		} else {
			fmt.Printf("💾 Cronograma exportado para iCalendar (.ics): %s\n", *icsPath)
		}
	}

	// 4. Sincroniza com Google Calendar ou gera link web fallback (PLUG-02)
	geminiDir := ""
	if info != nil {
		geminiDir = info.GeminiDir
	}
	if geminiDir == "" {
		home, _ := os.UserHomeDir()
		geminiDir = filepath.Join(home, ".gemini", "antigravity")
	}
	ctxName := *contextFlag
	if env := os.Getenv("AGYO_GOOGLE_CONTEXT"); env != "" && *contextFlag == "pessoal" {
		ctxName = env
	}
	authPath := workspace.AuthFilePathForContext(geminiDir, ctxName)
	if tok, err := workspace.LoadToken(authPath); err == nil && (!tok.IsExpired() || tok.RefreshToken != "") && !*webFlag {
		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		calClient := workspace.NewCalendarClient(nil, "", tokenGetter)
		createdCount := 0
		for _, evt := range calEvents {
			callCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, err := calClient.CreateEvent(callCtx, "primary", evt)
			cancel()
			if err == nil {
				createdCount++
			}
		}
		if createdCount > 0 {
			fmt.Printf("📅 %d eventos sincronizados no Google Calendar (contexto: %s).\n", createdCount, ctxName)
		}
	} else {
		examEvent := calEvents[len(calEvents)-1] // Dia da prova
		webURL := workspace.BuildCalendarWebURL(examEvent)
		fmt.Println("ℹ️  Google Calendar nao autenticado (ou flag --web informada).")
		fmt.Println("   Para sincronizar via terminal: agyo google auth")
		fmt.Printf("   Link web para adicionar o dia da prova no Google Calendar:\n   %s\n", webURL)
		if *webFlag {
			opened := false
			if profile.CheckStatus(info.BrowserProfile).IsRunning {
				if _, _, err := profile.OpenOrReuseTab(profile.DefaultDebugPort, webURL, false); err == nil {
					opened = true
				}
			}
			if !opened {
				_ = openBrowserURL(info.OS, webURL)
			}
		}
	}

	fmt.Println("✅ Cronograma de preparacao configurado com sucesso.")
}

func runStudyQuiz(info *platform.Info, args []string) {
	fs := flag.NewFlagSet("study quiz", flag.ContinueOnError)
	notebookID := fs.String("notebook", "", "ID do caderno do Google NotebookLM")
	topic := fs.String("topic", "", "Tema especifico para a pergunta do quiz")
	port := fs.Int("port", 0, "Porta DevTools do Chrome")

	if err := fs.Parse(args); err != nil {
		return
	}

	if *notebookID == "" {
		fmt.Fprintln(os.Stderr, "Erro: flag --notebook <id> e obrigatoria.")
		fmt.Fprintln(os.Stderr, "Uso: agyo study quiz --notebook <id> [--topic <tema>]")
		exitFunc(ExitUsageError)
		return
	}

	topicMsg := "sobre o conteudo geral do caderno"
	if *topic != "" {
		topicMsg = fmt.Sprintf("sobre o tema '%s'", *topic)
	}

	fmt.Printf("🎲 Solicitando pergunta de verificacao Feynman %s...\n\n", topicMsg)

	promptQuery := fmt.Sprintf("Elabore 1 pergunta desafiadora de verificacao %s com 4 alternativas (A, B, C, D) baseando-se estritamente nas fontes do caderno. Apresente o enunciado e as alternativas de forma clara. Nao revele a resposta correta ainda.", topicMsg)
	askRes, err := notebook.AskNotebook(info, *port, *notebookID, promptQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao gerar pergunta do quiz: %v\n", err)
		exitFunc(ExitRuntimeError)
		return
	}

	fmt.Println("==================================================")
	fmt.Println(askRes.Answer)
	fmt.Println("==================================================")
	fmt.Print("\nSua resposta (ou digite 'sair'): ")

	scanner := bufio.NewScanner(studyStdin)
	if !scanner.Scan() {
		return
	}
	studentAnswer := strings.TrimSpace(scanner.Text())
	if studentAnswer == "" || strings.EqualFold(studentAnswer, "sair") {
		fmt.Println("Sessao de quiz finalizada.")
		return
	}

	fmt.Println("\n🔍 Avaliando sua resposta com o tutor Feynman...")
	evalQuery := fmt.Sprintf("O estudante respondeu a questao anterior com: %q. Avalie se a resposta esta correta ou incorreta com base nas fontes. Se correta, parabenize e aprofunde brevemente o mecanismo conceitual. Se incorreta, explique didaticamente no metodo Feynman onde esteve o equivoco e apresente o raciocinio correto.", studentAnswer)
	evalRes, err := notebook.AskNotebook(info, *port, *notebookID, evalQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao obter feedback do tutor: %v\n", err)
		exitFunc(ExitRuntimeError)
		return
	}

	fmt.Println("\n--- Feedback do Tutor Feynman ---")
	fmt.Println(evalRes.Answer)
	fmt.Println("---------------------------------")
}

func generateFlashcards(text string) string {
	var sb strings.Builder
	sb.WriteString("# Flashcards de Estudo (Anki / Obsidian)\n\n")
	sb.WriteString("> Gerados automaticamente pelo agyo study via Google NotebookLM\n\n")

	// Procura padroes como "Pergunta: ... Resposta: ..." ou "1. ... ?"
	lines := strings.Split(text, "\n")
	var currentQ, currentA string

	reQ := regexp.MustCompile(`(?i)^(?:Pergunta|Q|\d+[\.\)])\s*:\s*(.+)$`)
	reA := regexp.MustCompile(`(?i)^(?:Resposta|A|Gabarito)\s*:\s*(.+)$`)

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if m := reQ.FindStringSubmatch(trim); len(m) > 1 {
			if currentQ != "" && currentA != "" {
				sb.WriteString(formatFlashcard(currentQ, currentA))
				currentQ, currentA = "", ""
			}
			currentQ = m[1]
		} else if m := reA.FindStringSubmatch(trim); len(m) > 1 {
			currentA = m[1]
		}
	}
	if currentQ != "" && currentA != "" {
		sb.WriteString(formatFlashcard(currentQ, currentA))
	} else if sb.Len() < 120 {
		// Fallback se nao houver tags explicitas: cria template de revisao
		sb.WriteString("## Cartao de Revisao Principal #flashcard\n\n")
		sb.WriteString("Q: Quais sao os conceitos fundamentais abordados nesta sintese?\n\n")
		sample := text
		if len(sample) > 500 {
			sample = sample[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("A: %s\n", strings.ReplaceAll(sample, "\n", " ")))
	}

	return sb.String()
}

func formatFlashcard(q, a string) string {
	return fmt.Sprintf("## Pergunta #flashcard\n\nQ: %s\n\nA: %s\n\n---\n\n", q, a)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func ifEmpty(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}
