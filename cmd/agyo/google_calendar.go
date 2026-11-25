package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

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
