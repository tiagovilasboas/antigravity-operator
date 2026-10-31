package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/installer"
	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

var openBrowserURL = func(osName, url string) error {
	if testing.Testing() || flag.Lookup("test.v") != nil {
		return nil // NUNCA abre navegador durante execuções de testes automatizados
	}
	if osName == "darwin" {
		return exec.Command("open", url).Start()
	} else if osName == "linux" {
		return exec.Command("xdg-open", url).Start()
	}
	return nil
}

func printGoogleUsage() {
	fmt.Println(`agyo google - Google Workspace Bridge (Calendar & Docs)
Autonomous assistant integration for calendar scheduling and document management.

Usage:
  agyo google <subcommand> [arguments]

Available subcommands:
  auth        Authenticate with Google Workspace via local loopback OAuth 2.0 (--credentials supported)
  status      Inspect Google Workspace authentication and token expiration
  calendar    List, schedule or export Google Calendar events (.ics & web template supported)
  docs        Read or create Google Docs documents
  mcp         Run or configure stdio MCP server for Google Calendar & Workspace (install|status|uninstall)

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
  add "<summary>" [--start "<datetime>"] [--end "<datetime>"] [--desc "<text>"] [--loc "<text>"] [--ics "<file.ics>"] [--web]
      Schedule a new event in the primary calendar (supports natural dates, .ics export and web zero-auth fallback)
  ics "<summary>" [--start "<datetime>"] [--end "<datetime>"] [--desc "<text>"] [--loc "<text>"] [--out "<file.ics>"]
      Export a universal iCalendar file directly without network access`)
}

func printGoogleMcpUsage() {
	fmt.Println(`Usage:
  agyo google mcp [action]

Actions:
  install     Configure google-calendar MCP server in Antigravity (mcp_config.json & default-servers.json)
  status      Check whether google-calendar MCP server is configured and active
  uninstall   Deregister google-calendar MCP server from Antigravity

Without action, 'agyo google mcp' starts the stdio MCP server for direct agent communication with zero browser tabs.`)
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
			fmt.Println("Usage: agyo google auth [--credentials <file.json>] [--client-id <id>] [--client-secret <secret>] [--port <p>]")
			fmt.Println("Inicia o fluxo OAuth 2.0 loopback local para autenticação com o Google Workspace.")
			return
		}
		authCmd := flag.NewFlagSet("google auth", flag.ExitOnError)
		clientID := authCmd.String("client-id", "", "Google OAuth 2.0 Client ID")
		clientSecret := authCmd.String("client-secret", "", "Google OAuth 2.0 Client Secret")
		credentialsFile := authCmd.String("credentials", "", "Caminho do arquivo credentials.json baixado do Google Cloud")
		port := authCmd.Int("port", workspace.DefaultOAuthPort, "Porta do servidor loopback local")
		_ = authCmd.Parse(args[1:])

		cid := *clientID
		csec := *clientSecret

		if *credentialsFile != "" {
			creds, err := workspace.ParseGoogleCredentials(*credentialsFile)
			if err != nil {
				runtimeError(err, "Falha ao validar arquivo de credenciais Google Cloud")
				return
			}
			if cid == "" {
				cid = creds.ClientID
			}
			if csec == "" {
				csec = creds.ClientSecret
			}
			fmt.Printf("📄 Credenciais importadas com sucesso de: %s\n", *credentialsFile)
			fmt.Printf("   Client ID     : %s\n", cid)
			fmt.Printf("   Client Secret : %s\n", workspace.MaskSecret(csec))
		}

		if cid == "" {
			cid = os.Getenv("GOOGLE_WORKSPACE_CLIENT_ID")
		}
		if csec == "" {
			csec = os.Getenv("GOOGLE_WORKSPACE_CLIENT_SECRET")
		}

		if cid == "" {
			fmt.Println("ℹ️  Dica: Você pode fornecer suas credenciais via --credentials <arquivo.json>, --client-id ou GOOGLE_WORKSPACE_CLIENT_ID.")
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

		openedInProfile := false
		if profile.CheckStatus(info.BrowserProfile).IsRunning {
			if _, _, err := profile.OpenOrReuseTab(profile.DefaultDebugPort, authURL, false); err == nil {
				openedInProfile = true
			}
		}

		if !openedInProfile {
			_ = openBrowserURL(info.OS, authURL)
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
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			mcpAction := args[1]
			switch mcpAction {
			case "install":
				st, err := installer.InstallCalendarMCP(info)
				if err != nil {
					runtimeError(err, "Falha ao instalar MCP do Google Calendar")
					return
				}
				fmt.Println("✅ Servidor MCP 'google-calendar' instalado e configurado no Antigravity!")
				fmt.Printf("   Comando : %s %s\n", st.Command, strings.Join(st.Args, " "))
				fmt.Printf("   Schemas : %d ferramentas registradas em %s\n", st.SchemasCount, st.SchemaDir)
				fmt.Println("💡 O agente de IA agora tem acesso direto ao Google Calendar com zero abas no navegador.")
				return

			case "status":
				st, err := installer.CalendarMCPStatus(info)
				if err != nil {
					runtimeError(err, "Falha ao verificar status do MCP")
					return
				}
				if hasJSON(args[2:]) {
					data, _ := json.MarshalIndent(st, "", "  ")
					fmt.Println(string(data))
					return
				}
				if st.Installed {
					fmt.Println("🔌 Servidor MCP Google Calendar: CONFIGURADO")
					fmt.Printf("   Nome      : %s\n", st.Name)
					fmt.Printf("   Comando   : %s %s\n", st.Command, strings.Join(st.Args, " "))
					fmt.Printf("   Schemas   : %d ferramentas ativas em %s\n", st.SchemasCount, st.SchemaDir)
					fmt.Println("   Status    : Ativo e pronto para uso pelo Antigravity (zero abas)")
				} else {
					fmt.Println("🔌 Servidor MCP Google Calendar: NÃO CONFIGURADO")
					fmt.Println("   Para instalar : agyo google mcp install")
				}
				return

			case "uninstall":
				if err := installer.UninstallCalendarMCP(info); err != nil {
					runtimeError(err, "Falha ao desinstalar MCP do Google Calendar")
					return
				}
				fmt.Println("🗑️  Servidor MCP 'google-calendar' desinstalado do Antigravity.")
				return

			default:
				usageError(fmt.Sprintf("unknown mcp action %q (use 'agyo google mcp --help')", mcpAction))
				return
			}
		}

		if hasHelp(args[1:]) {
			printGoogleMcpUsage()
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
			fmt.Println("Usage: agyo google calendar add \"<summary>\" [--start \"<datetime>\"] [--end \"<datetime>\"] [--desc \"<description>\"] [--loc \"<location>\"] [--ics \"<file.ics>\"] [--web]")
			fmt.Println("Agenda um novo evento no calendário primário do Google Calendar.")
			return
		}
		addCmd := flag.NewFlagSet("calendar add", flag.ExitOnError)
		startStr := addCmd.String("start", "", "Horário de início (RFC3339, ISO, DD/MM/YYYY ou relativo ex: 'hoje 15:00', '+2h')")
		endStr := addCmd.String("end", "", "Horário de término (RFC3339, ISO, DD/MM/YYYY ou relativo ex: '+1h')")
		desc := addCmd.String("desc", "", "Descrição do evento")
		loc := addCmd.String("loc", "", "Localização ou link de videoconferência")
		icsPath := addCmd.String("ics", "", "Exportar evento para arquivo iCalendar universal (.ics)")
		webFlag := addCmd.Bool("web", false, "Gerar URL web do Google Calendar para agendamento direto no navegador")
		_ = addCmd.Parse(permuteArgs(args[1:], nil))

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
			startTime, err = workspace.ParseFlexTime(*startStr, time.Now())
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --start: %v", err))
				return
			}
		}

		var endTime time.Time
		if *endStr != "" {
			var err error
			endTime, err = workspace.ParseFlexTime(*endStr, startTime)
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --end: %v", err))
				return
			}
		} else {
			endTime = startTime.Add(1 * time.Hour)
		}

		event := workspace.CalendarEvent{
			Summary:     summary,
			Description: *desc,
			Location:    *loc,
			Start:       startTime,
			End:         endTime,
		}

		if *icsPath != "" {
			cwd, _ := os.Getwd()
			baseDir := cwd
			if filepath.IsAbs(*icsPath) {
				baseDir = filepath.Dir(*icsPath)
			}
			if err := workspace.WriteSecureICSFile(*icsPath, []byte(event.ToICS()), baseDir); err != nil {
				runtimeError(err, "Falha ao exportar arquivo ICS seguro")
				return
			}
			fmt.Printf("💾 Arquivo iCalendar (.ics) exportado com sucesso: %s\n", *icsPath)
		}

		if *webFlag {
			webURL := workspace.BuildCalendarWebURL(event)
			fmt.Println("🌐 Agendamento Web do Google Calendar (Zero-Auth):")
			fmt.Printf("   %s\n", webURL)
			opened := false
			if profile.CheckStatus(info.BrowserProfile).IsRunning {
				if _, _, err := profile.OpenOrReuseTab(profile.DefaultDebugPort, webURL, false); err == nil {
					opened = true
				}
			}
			if !opened {
				_ = openBrowserURL(info.OS, webURL)
			}
			fmt.Println("💡 Confirme a criação do evento na aba aberta do navegador.")
			return
		}

		tokenGetter := func(ctx context.Context) (string, error) {
			return getGoogleWorkspaceToken(info, authPath)
		}
		client := workspace.NewCalendarClient(nil, "", tokenGetter)

		created, err := client.CreateEvent(context.Background(), "primary", event)
		if err != nil {
			webURL := workspace.BuildCalendarWebURL(event)
			fmt.Println("⚠️  Não foi possível agendar via API direta (Google Workspace não autenticado ou erro na API).")
			fmt.Println("🌐 Fallback Zero-Auth ativado: use o link abaixo para criar o evento no navegador:")
			fmt.Printf("   %s\n", webURL)
			fmt.Println("💡 Acesse o link acima no navegador ou autentique o terminal com: agyo google auth")
			return
		}

		fmt.Printf("✅ Evento agendado com sucesso no Google Calendar!\n")
		fmt.Printf("   Título  : %s\n", created.Summary)
		fmt.Printf("   Horário : %s - %s\n", created.Start.Format("2006-01-02 15:04"), created.End.Format("15:04"))
		if created.HtmlLink != "" {
			fmt.Printf("   Link    : %s\n", created.HtmlLink)
		}

	case "ics":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo google calendar ics \"<summary>\" [--start \"<datetime>\"] [--end \"<datetime>\"] [--desc \"<description>\"] [--loc \"<location>\"] [--out \"<file.ics>\"]")
			fmt.Println("Exporta um evento universal no formato iCalendar (.ics) com proteção contra CRLF injection e Path Traversal.")
			return
		}
		icsCmd := flag.NewFlagSet("calendar ics", flag.ExitOnError)
		startStr := icsCmd.String("start", "", "Horário de início (RFC3339, ISO, DD/MM/YYYY ou relativo ex: 'hoje 15:00', '+2h')")
		endStr := icsCmd.String("end", "", "Horário de término (RFC3339, ISO, DD/MM/YYYY ou relativo ex: '+1h')")
		desc := icsCmd.String("desc", "", "Descrição do evento")
		loc := icsCmd.String("loc", "", "Localização ou link de videoconferência")
		outPath := icsCmd.String("out", "evento.ics", "Caminho do arquivo .ics gerado")
		_ = icsCmd.Parse(permuteArgs(args[1:], nil))

		summary := ""
		if icsCmd.NArg() > 0 {
			summary = icsCmd.Arg(0)
		}
		if summary == "" {
			usageError("o título do evento é obrigatório: agyo google calendar ics \"<título>\"")
			return
		}

		startTime := time.Now()
		if *startStr != "" {
			var err error
			startTime, err = workspace.ParseFlexTime(*startStr, time.Now())
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --start: %v", err))
				return
			}
		}

		var endTime time.Time
		if *endStr != "" {
			var err error
			endTime, err = workspace.ParseFlexTime(*endStr, startTime)
			if err != nil {
				usageError(fmt.Sprintf("formato de data/hora inválido para --end: %v", err))
				return
			}
		} else {
			endTime = startTime.Add(1 * time.Hour)
		}

		event := workspace.CalendarEvent{
			Summary:     summary,
			Description: *desc,
			Location:    *loc,
			Start:       startTime,
			End:         endTime,
		}

		cwd, _ := os.Getwd()
		baseDir := cwd
		if filepath.IsAbs(*outPath) {
			baseDir = filepath.Dir(*outPath)
		}
		if err := workspace.WriteSecureICSFile(*outPath, []byte(event.ToICS()), baseDir); err != nil {
			runtimeError(err, "Falha ao gravar arquivo ICS seguro")
			return
		}
		fmt.Printf("✅ Arquivo iCalendar exportado com sucesso: %s\n", *outPath)

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
		_ = readCmd.Parse(permuteArgs(args[1:], map[string]bool{"json": true}))

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
		_ = createCmd.Parse(permuteArgs(args[1:], map[string]bool{"json": true}))

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
