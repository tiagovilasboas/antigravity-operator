package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

func printGoogleUsage() {
	fmt.Println(`agyo google - Google Workspace Bridge (Calendar & Docs)
Autonomous assistant integration for calendar scheduling and document management.

Usage:
  agyo google <subcommand> [arguments]

Available subcommands:
  auth        Authenticate with Google Workspace via local loopback OAuth 2.0
  status      Inspect Google Workspace authentication and token expiration
  calendar    List or schedule Google Calendar events
  docs        Read or create Google Docs documents
  mcp         Run stdio MCP server exposing Google Workspace tools

Aliases:
  agyo gsuite <subcommand>
  agyo workspace <subcommand>

Use 'agyo google <subcommand> --help' for details on each subcommand.`)
}

func printGoogleCalendarUsage() {
	fmt.Println(`Usage:
  agyo google calendar <action> [arguments]

Actions:
  list [--today|--week] [--max N] [--json]
      List upcoming Google Calendar events
  add "<summary>" [--start "<datetime>"] [--end "<datetime>"] [--desc "<text>"] [--loc "<text>"]
      Schedule a new event in the primary calendar (start format: RFC3339)`)
}

func printGoogleDocsUsage() {
	fmt.Println(`Usage:
  agyo google docs <action> [arguments]

Actions:
  read <documentId-or-url> [--json]
      Read structured text from a Google Doc
  create "<title>" [--content "<text>"] [--json]
      Create a new Google Doc with optional initial text`)
}

func getGoogleWorkspaceToken(info *platform.Info, authPath string) (string, error) {
	tok, err := workspace.LoadToken(authPath)
	if err != nil {
		return "", fmt.Errorf("Google Workspace não autenticado. Execute 'agyo google auth' no terminal: %w", err)
	}
	if tok.IsExpired() && tok.RefreshToken != "" {
		refreshed, err := workspace.RefreshAccessToken(context.Background(), nil, "", tok)
		if err == nil {
			_ = workspace.SaveToken(authPath, refreshed)
			return refreshed.AccessToken, nil
		}
	}
	return tok.AccessToken, nil
}

func runGoogle(info *platform.Info, args []string) {
	contextName := os.Getenv("AGYO_GOOGLE_CONTEXT")
	var filtered []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--context" && i+1 < len(args) {
			contextName = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--context=") {
			contextName = strings.TrimPrefix(args[i], "--context=")
		} else {
			filtered = append(filtered, args[i])
		}
	}
	args = filtered

	if len(args) == 0 {
		printGoogleUsage()
		exitFunc(ExitUsageError)
		return
	}

	sub := args[0]
	if isHelp(sub) {
		printGoogleUsage()
		return
	}

	authPath := workspace.AuthFilePathForContext(info.GeminiDir, contextName)

	switch sub {
	case "auth":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google auth [--client-id <id>] [--client-secret <secret>] [--port <p>]")
			fmt.Println("Inicia o fluxo OAuth 2.0 loopback local para autenticação com o Google Workspace.")
			return
		}
		authCmd := flag.NewFlagSet("google auth", flag.ExitOnError)
		clientID := authCmd.String("client-id", "", "Google OAuth 2.0 Client ID")
		clientSecret := authCmd.String("client-secret", "", "Google OAuth 2.0 Client Secret")
		port := authCmd.Int("port", workspace.DefaultOAuthPort, "Porta do servidor loopback local")
		_ = authCmd.Parse(args[1:])

		cid := *clientID
		if cid == "" {
			cid = os.Getenv("GOOGLE_WORKSPACE_CLIENT_ID")
		}
		csec := *clientSecret
		if csec == "" {
			csec = os.Getenv("GOOGLE_WORKSPACE_CLIENT_SECRET")
		}

		if cid == "" {
			fmt.Println("ℹ️  Dica: Você pode fornecer seu Client ID via --client-id ou GOOGLE_WORKSPACE_CLIENT_ID.")
			cid = "antigravity-operator.apps.googleusercontent.com"
		}

		state := workspace.GenerateState()
		lb, err := workspace.StartLoopbackServer(*port, state)
		if err != nil {
			runtimeError(err, fmt.Sprintf("Verifique se a porta %d está livre", *port))
			return
		}
		defer lb.Close()

		authURL := workspace.BuildAuthURL("", cid, lb.RedirectURI(), state, nil)
		fmt.Println("🌐 Conectando ao Google Workspace (Calendar & Docs)...")
		fmt.Printf("   Acesse a URL abaixo no seu navegador para autorizar:\n\n   %s\n\n", authURL)
		fmt.Println("⏳ Aguardando confirmação no navegador (pressione Ctrl+C para cancelar)...")

		if info.OS == "darwin" {
			_ = exec.Command("open", authURL).Start()
		} else if info.OS == "linux" {
			_ = exec.Command("xdg-open", authURL).Start()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		code, err := lb.WaitForCode(ctx)
		if err != nil {
			runtimeError(err, "Falha ao capturar o código de autorização")
			return
		}

		fmt.Println("🔑 Trocando código por tokens de acesso...")
		tok, err := workspace.ExchangeCode(ctx, nil, "", cid, csec, code, lb.RedirectURI())
		if err != nil {
			runtimeError(err, "Falha na troca do token OAuth")
			return
		}

		tok.Context = contextName
		if err := workspace.SaveToken(authPath, tok); err != nil {
			runtimeError(err, "")
			return
		}

		fmt.Printf("✅ Google Workspace autenticado com sucesso!\n")
		fmt.Printf("   Credenciais salvas em: %s (chmod 0600)\n", authPath)
		if contextName != "" {
			fmt.Printf("   Contexto ativo       : %s\n", contextName)
		}
		fmt.Println("💡 Você já pode usar: agyo google calendar list ou agyo google docs read")

	case "status":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google status [--context <name>] [--json]")
			fmt.Println("Verifica se há credenciais válidas do Google Workspace salvas localmente.")
			return
		}
		tok, err := workspace.LoadToken(authPath)
		if hasJSON(args[1:]) {
			st := map[string]interface{}{
				"authenticated": err == nil,
				"token_path":    authPath,
				"is_expired":    tok != nil && tok.IsExpired(),
			}
			if contextName != "" {
				st["context"] = contextName
			}
			if tok != nil {
				st["expiry"] = tok.Expiry
				st["scopes"] = tok.Scopes
			}
			data, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(data))
			return
		}

		if err != nil {
			fmt.Println("⚠️  Google Workspace NÃO autenticado.")
			fmt.Printf("   Arquivo não encontrado: %s\n", authPath)
			if contextName != "" {
				fmt.Printf("   Contexto verificado   : %s\n", contextName)
			}
			fmt.Println("   Para conectar sua conta: agyo google auth")
			return
		}

		fmt.Println("✅ Google Workspace AUTENTICADO!")
		fmt.Printf("   Arquivo de credenciais: %s\n", authPath)
		if contextName != "" {
			fmt.Printf("   Contexto ativo        : %s\n", contextName)
		}
		if tok.IsExpired() {
			fmt.Printf("   Token de acesso: Expirado (será renovado automaticamente via refresh token)\n")
		} else {
			fmt.Printf("   Token de acesso: Válido (expira em: %s)\n", tok.Expiry.Format("2006-01-02 15:04:05"))
		}
		if len(tok.Scopes) > 0 {
			fmt.Printf("   Escopos ativos : %s\n", strings.Join(tok.Scopes, ", "))
		}

	case "calendar":
		runGoogleCalendar(info, authPath, args[1:])

	case "docs":
		runGoogleDocs(info, authPath, args[1:])

	case "mcp":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google mcp")
			fmt.Println("Inicia o servidor MCP stdio expondo ferramentas do Google Workspace e NotebookLM.")
			return
		}
		if err := notebook.ServeMCP(info, os.Stdin, os.Stdout); err != nil {
			runtimeError(err, "")
			return
		}

	default:
		usageError(fmt.Sprintf("unknown google subcommand %q (run 'agyo google --help')", sub))
	}
}

func runGoogleCalendar(info *platform.Info, authPath string, args []string) {
	if len(args) == 0 || isHelp(args[0]) {
		printGoogleCalendarUsage()
		if len(args) == 0 {
			exitFunc(ExitUsageError)
		}
		return
	}

	action := args[0]
	switch action {
	case "list":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google calendar list [--today|--week] [--max N] [--json]")
			fmt.Println("Lista os próximos eventos agendados no calendário primário do Google Calendar.")
			return
		}
		listCmd := flag.NewFlagSet("calendar list", flag.ExitOnError)
		today := listCmd.Bool("today", false, "Filtrar apenas eventos de hoje")
		week := listCmd.Bool("week", false, "Filtrar eventos dos próximos 7 dias")
		max := listCmd.Int("max", 20, "Número máximo de eventos")
		jsonOut := listCmd.Bool("json", false, "Saída em formato JSON")
		_ = listCmd.Parse(args[1:])

		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		client := workspace.NewCalendarClient(nil, "", tokenGetter)

		now := time.Now()
		var timeMin, timeMax time.Time
		timeMin = now
		if *today {
			timeMin = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			timeMax = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		} else if *week {
			timeMax = now.Add(7 * 24 * time.Hour)
		}

		events, err := client.ListEvents(context.Background(), "primary", timeMin, timeMax, *max)
		if err != nil {
			runtimeError(err, "Execute 'agyo google auth' se as credenciais expiraram")
			return
		}

		if *jsonOut || hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(events, "", "  ")
			fmt.Println(string(data))
			return
		}

		if len(events) == 0 {
			fmt.Println("ℹ️  Nenhum evento agendado encontrado no período.")
			return
		}

		fmt.Printf("📅 Próximos Eventos do Google Calendar (%d encontrados):\n", len(events))
		fmt.Println("-----------------------------------------------------------------")
		for _, e := range events {
			timeRange := e.Start.Format("2006-01-02 15:04")
			if !e.End.IsZero() {
				timeRange += " - " + e.End.Format("15:04")
			}
			loc := ""
			if e.Location != "" {
				loc = fmt.Sprintf(" (%s)", e.Location)
			}
			fmt.Printf("• [%s] %s%s\n", timeRange, e.Summary, loc)
			if e.Description != "" {
				descLine := strings.ReplaceAll(e.Description, "\n", " ")
				if len(descLine) > 70 {
					descLine = descLine[:67] + "..."
				}
				fmt.Printf("    📝 %s\n", descLine)
			}
		}
		fmt.Println("-----------------------------------------------------------------")

	case "add":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google calendar add \"<summary>\" [--start \"<datetime>\"] [--end \"<datetime>\"] [--desc \"<description>\"] [--loc \"<location>\"]")
			fmt.Println("Agenda um novo evento no calendário primário do Google Calendar.")
			return
		}
		addCmd := flag.NewFlagSet("calendar add", flag.ExitOnError)
		startStr := addCmd.String("start", "", "Horário de início (formato RFC3339, ex: 2026-10-15T14:00:00Z)")
		endStr := addCmd.String("end", "", "Horário de término (formato RFC3339)")
		desc := addCmd.String("desc", "", "Descrição do evento")
		loc := addCmd.String("loc", "", "Localização ou link de videoconferência")
		_ = addCmd.Parse(args[1:])

		summary := ""
		if addCmd.NArg() > 0 {
			summary = addCmd.Arg(0)
		}
		if summary == "" {
			usageError("o título do evento é obrigatório: agyo google calendar add \"<título>\"")
			return
		}

		startTime := time.Now()
		if *startStr != "" {
			var err error
			startTime, err = time.Parse(time.RFC3339, *startStr)
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --start (use RFC3339, ex: 2026-10-15T14:00:00Z): %v", err))
				return
			}
		}

		var endTime time.Time
		if *endStr != "" {
			var err error
			endTime, err = time.Parse(time.RFC3339, *endStr)
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --end: %v", err))
				return
			}
		} else {
			endTime = startTime.Add(1 * time.Hour)
		}

		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		client := workspace.NewCalendarClient(nil, "", tokenGetter)

		created, err := client.CreateEvent(context.Background(), "primary", workspace.CalendarEvent{
			Summary:     summary,
			Description: *desc,
			Location:    *loc,
			Start:       startTime,
			End:         endTime,
		})
		if err != nil {
			runtimeError(err, "")
			return
		}

		fmt.Printf("✅ Evento agendado com sucesso no Google Calendar!\n")
		fmt.Printf("   Título  : %s\n", created.Summary)
		fmt.Printf("   Horário : %s - %s\n", created.Start.Format("2006-01-02 15:04"), created.End.Format("15:04"))
		if created.HtmlLink != "" {
			fmt.Printf("   Link    : %s\n", created.HtmlLink)
		}

	default:
		usageError(fmt.Sprintf("unknown calendar action %q (run 'agyo google calendar --help')", action))
	}
}

func runGoogleDocs(info *platform.Info, authPath string, args []string) {
	if len(args) == 0 || isHelp(args[0]) {
		printGoogleDocsUsage()
		if len(args) == 0 {
			exitFunc(ExitUsageError)
		}
		return
	}

	action := args[0]
	switch action {
	case "read":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google docs read <documentId-or-url> [--json]")
			fmt.Println("Lê o conteúdo em texto estruturado de um documento do Google Docs.")
			return
		}
		readCmd := flag.NewFlagSet("docs read", flag.ExitOnError)
		jsonOut := readCmd.Bool("json", false, "Saída em formato JSON")
		_ = readCmd.Parse(args[1:])

		docIDOrURL := ""
		if readCmd.NArg() > 0 {
			docIDOrURL = readCmd.Arg(0)
		}
		if docIDOrURL == "" {
			usageError("informe o ID ou URL do Google Doc: agyo google docs read <id-ou-url>")
			return
		}

		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		client := workspace.NewDocsClient(nil, "", tokenGetter)

		doc, err := client.GetDocument(context.Background(), docIDOrURL)
		if err != nil {
			runtimeError(err, "Verifique se o documento existe e se a conta possui permissão de leitura")
			return
		}

		if *jsonOut || hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(doc, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("📄 Google Doc: %s (ID: %s)\n", doc.Title, doc.ID)
		if doc.URL != "" {
			fmt.Printf("   Link: %s\n", doc.URL)
		}
		fmt.Println("-----------------------------------------------------------------")
		fmt.Println(doc.BodyText)
		fmt.Println("-----------------------------------------------------------------")

	case "create":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google docs create \"<title>\" [--content \"<text>\"] [--json]")
			fmt.Println("Cria um novo documento no Google Docs com título e conteúdo inicial opcional.")
			return
		}
		createCmd := flag.NewFlagSet("docs create", flag.ExitOnError)
		content := createCmd.String("content", "", "Conteúdo textual inicial do documento")
		jsonOut := createCmd.Bool("json", false, "Saída em formato JSON")
		_ = createCmd.Parse(args[1:])

		title := ""
		if createCmd.NArg() > 0 {
			title = createCmd.Arg(0)
		}
		if title == "" {
			usageError("o título do documento é obrigatório: agyo google docs create \"<título>\"")
			return
		}

		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		client := workspace.NewDocsClient(nil, "", tokenGetter)

		doc, err := client.CreateDocument(context.Background(), title, *content)
		if err != nil {
			runtimeError(err, "")
			return
		}

		if *jsonOut || hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(doc, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("✅ Documento criado com sucesso no Google Docs!\n")
		fmt.Printf("   Título : %s\n", doc.Title)
		fmt.Printf("   ID     : %s\n", doc.ID)
		fmt.Printf("   URL    : %s\n", doc.URL)

	default:
		usageError(fmt.Sprintf("unknown docs action %q (run 'agyo google docs --help')", action))
	}
}
