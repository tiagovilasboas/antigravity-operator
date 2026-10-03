package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tiagoboas/antigravity-operator/internal/checkpoint"
	"github.com/tiagoboas/antigravity-operator/internal/completion"
	"github.com/tiagoboas/antigravity-operator/internal/dashboard"
	"github.com/tiagoboas/antigravity-operator/internal/doctor"
	"github.com/tiagoboas/antigravity-operator/internal/exporter"
	"github.com/tiagoboas/antigravity-operator/internal/hook"
	"github.com/tiagoboas/antigravity-operator/internal/installer"
	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/internal/profile"
	"github.com/tiagoboas/antigravity-operator/internal/session"
	"github.com/tiagoboas/antigravity-operator/internal/watcher"
)

// Version is set at build time from the git tag:
//
//	go build -ldflags "-X main.Version=0.4.5" ./cmd/agyo
//
// Builds without the flag report "dev".
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	info, err := platform.Detect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting platform: %v\n", err)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		runInit(os.Args[2:])
	case "session":
		runSession(info, os.Args[2:])
	case "checkpoint":
		runCheckpoint(os.Args[2:])
	case "rollback":
		runRollback(os.Args[2:])
	case "dashboard":
		runDashboard(info, os.Args[2:])
	case "doctor":
		runDoctor(info, os.Args[2:])
	case "browser":
		runBrowser(info, os.Args[2:])
	case "sync":
		runSync(info)
	case "hook":
		runHook(os.Args[2:])
	case "completion":
		runCompletion(os.Args[2:])
	case "about":
		printAbout()
	case "version", "-v", "--version":
		fmt.Printf("agyo (Antigravity Operator) v%s [%s/%s]\n", Version, info.OS, info.Arch)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`agyo - Antigravity Operator CLI (Autonomous Session Agent Engine)
The Official Community Outer Harness for Google Antigravity & Google AI Pro

Usage:
  agyo <command> [arguments]

Available commands:
  init [dir]          Scaffold operational memory (.agents/session/) in the target project
  session status      Display active session objective, status, and task completion metrics
  session compact     Archive completed tasks and rollup active todo.md to avoid context bloat
  session archive     Archive completed session to historical log and reset templates
  session list        List all archived historical sessions
  session restore [f] Restore a past archived session into active memory with automatic backup
  session watch       Stream active agent reasoning and desktop notifications in real time
  session export      Export consolidated session report in markdown or HTML format
  checkpoint [name]   Create an atomic filesystem snapshot before risky refactoring
  rollback [id]       Safely undo agent edits and restore exact working tree from a checkpoint
  dashboard           Launch local web dashboard for live monitoring and DevTools inspection
  doctor [--fix]      Audit host readiness or auto-repair session memory (supports --json)
  browser start       Launch isolated Chrome instance with remote debugging flags
  browser status      Inspect DevTools port (9222) readiness and Chrome process PID
  browser stop        Gracefully terminate isolated Chrome process (SIGTERM)
  browser tabs        List all open tabs and target IDs in the isolated Chrome
  browser open <url>  Open a new tab at the given URL in the isolated Chrome
  browser close <id>  Close a specific tab by ID
  browser eval "<js>" Evaluate JavaScript expression in the active tab (pure Go CDP)
  browser shot [file] Capture PNG screenshot of active tab without external libraries
  sync                Synchronize canonical rules and MCP manifests to Google Antigravity
  hook install [dir]  Install git pre-commit hook to safeguard session continuity
  hook uninstall [dir] Remove agyo git pre-commit hook
  completion [shell]  Generate shell autocompletion script (bash, zsh, fish)
  about               Display manifesto and tribute to the community & Google AI Pro
  version             Print version and system architecture`)
}

func printAbout() {
	fmt.Println(`================================================================================
  ANTIGRAVITY OPERATOR (agyo) — SESSION RUNTIME & OUTER HARNESS
================================================================================

This project is an open engineering tribute to the developer community, students,
and researchers worldwide, and a special thank you to Google for the transformative
student access program through Google AI Pro.

Mission:
Transform the raw atomic power of Google Antigravity into an autonomous, safe, and
persistent Session Operator with filesystem memory (.agents/session/) and seamless
parity across macOS and Linux — empowering every student and engineer to leverage
100% of their Gemini Pro quota without token waste or runtime friction.

Built with care, precision, and canonical software engineering (Martin Fowler Outer Harness).
================================================================================`)
}

func runInit(args []string) {
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	force := initCmd.Bool("force", false, "Overwrite existing session files")
	_ = initCmd.Parse(args)

	targetDir := "."
	if initCmd.NArg() > 0 {
		targetDir = initCmd.Arg(0)
	}

	res, err := session.Init(targetDir, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing session: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Session memory initialized at: %s\n", res.SessionDir)
	for _, c := range res.Created {
		fmt.Printf("   + Created: %s\n", c)
	}
	for _, s := range res.Skipped {
		fmt.Printf("   - Kept (already exists): %s\n", s)
	}
}

func runSession(info *platform.Info, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: agyo session [status|compact|archive|watch|export] [dir]")
		os.Exit(1)
	}

	sub := args[0]
	targetDir := "."

	switch sub {
	case "status":
		statusCmd := flag.NewFlagSet("session status", flag.ExitOnError)
		jsonOut := statusCmd.Bool("json", false, "Output session status as JSON")
		_ = statusCmd.Parse(args[1:])
		if statusCmd.NArg() > 0 {
			targetDir = statusCmd.Arg(0)
		}

		sum, err := session.GetSummary(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if *jsonOut {
			data, _ := json.MarshalIndent(sum, "", "  ")
			fmt.Println(string(data))
			return
		}

		pct := 0
		if sum.TotalTasks > 0 {
			pct = (sum.DoneTasks * 100) / sum.TotalTasks
		}

		fmt.Println("📋 Active Session Overview (.agents/session/)")
		fmt.Println("-----------------------------------------------------------------")
		fmt.Printf("🎯 Objective : %s\n", sum.Objective)
		fmt.Printf("⚡ Phase     : %s\n", sum.Status)
		fmt.Printf("📊 Progress  : %d/%d tasks completed (%d%%)\n", sum.DoneTasks, sum.TotalTasks, pct)
		fmt.Printf("🧠 Footprint : %d B (~%d tokens) [%s]\n", sum.TotalSizeBytes, sum.EstimatedTokens, sum.HealthStatus)
		if len(sum.Pending) > 0 {
			fmt.Println("\n⏳ Pending Next Steps:")
			for i, p := range sum.Pending {
				if i >= 5 {
					fmt.Printf("   ... and %d more\n", len(sum.Pending)-i)
					break
				}
				fmt.Printf("   - [ ] %s\n", p)
			}
		}
		if sum.HealthStatus == "Bloated 🔴" || sum.DoneTasks >= 5 {
			fmt.Println()
			fmt.Printf("💡 Tip: %d completed tasks / ~%d tokens in session. Run 'agyo session compact' to reduce context bloat.\n", sum.DoneTasks, sum.EstimatedTokens)
		}
		fmt.Println("-----------------------------------------------------------------")

	case "compact":
		runSessionCompact(args[1:])

	case "archive":
		if len(args) > 1 {
			targetDir = args[1]
		}
		archiveFile, err := session.Archive(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error archiving session: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📦 Session archived successfully to:\n   %s\n", archiveFile)
		fmt.Println("✅ State and tasks reset with clean templates for the next task!")

	case "list":
		listCmd := flag.NewFlagSet("session list", flag.ExitOnError)
		jsonOut := listCmd.Bool("json", false, "Output archives list as JSON")
		_ = listCmd.Parse(args[1:])
		if listCmd.NArg() > 0 {
			targetDir = listCmd.Arg(0)
		}

		archives, err := session.ListArchives(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing archived sessions: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			data, _ := json.MarshalIndent(archives, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(archives) == 0 {
			fmt.Println("📁 Nenhuma sessão arquivada encontrada em .agents/session/archive/")
			return
		}
		fmt.Printf("📚 Sessões Arquivadas (%d encontradas):\n", len(archives))
		fmt.Println("-----------------------------------------------------------------")
		for _, a := range archives {
			fmt.Printf("📦 %s\n", a.Filename)
			fmt.Printf("   🎯 Objetivo : %s\n", a.Objective)
			fmt.Printf("   📊 Tarefas  : %d concluídas de %d\n", a.TasksDone, a.TasksTotal)
			fmt.Printf("   💾 Tamanho  : %d bytes\n\n", a.SizeBytes)
		}
		fmt.Println("💡 Para restaurar uma sessão: agyo session restore <nome-do-arquivo>")

	case "restore":
		archiveName := ""
		if len(args) == 2 {
			if fi, err := os.Stat(args[1]); err == nil && fi.IsDir() {
				targetDir = args[1]
			} else {
				archiveName = args[1]
			}
		} else if len(args) > 2 {
			archiveName = args[1]
			targetDir = args[2]
		}
		res, err := session.Restore(targetDir, archiveName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error restoring session: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("♻️  Sessão restaurada com sucesso a partir de:\n   %s\n", res.RestoredFile)
		if res.BackupFile != "" {
			fmt.Printf("🛡️  Backup preventivo da sessão anterior salvo em:\n   %s\n", res.BackupFile)
		}
		fmt.Printf("🎯 Objetivo restaurado : %s\n", res.Objective)
		fmt.Printf("📋 Tarefas restauradas  : %d\n", res.TasksCount)

	case "watch", "tail":
		runSessionWatch(info, args[1:])

	case "export":
		runSessionExport(info, args[1:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown session subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func runSessionCompact(args []string) {
	compactCmd := flag.NewFlagSet("session compact", flag.ExitOnError)
	threshold := compactCmd.Int("threshold", 5, "Minimum completed tasks to trigger compaction")
	keep := compactCmd.Int("keep", 3, "Number of recent completed tasks to retain in active todo.md")
	dryRun := compactCmd.Bool("dry-run", false, "Preview compaction and token savings without modifying disk")
	_ = compactCmd.Parse(args)

	targetDir := "."
	if compactCmd.NArg() > 0 {
		targetDir = compactCmd.Arg(0)
	}

	res, err := session.Compact(targetDir, session.CompactOptions{
		Threshold: *threshold,
		KeepLast:  *keep,
		DryRun:    *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compacting session: %v\n", err)
		os.Exit(1)
	}

	if res.AlreadyCompact {
		fmt.Println("✨ Session memory is already clean and compact!")
		fmt.Printf("   Active completed tasks: %d (compaction threshold is %d)\n", res.RetainedTasks, *threshold)
		return
	}

	mode := "Applied"
	if *dryRun {
		mode = "Dry Run (Preview)"
	}

	fmt.Printf("🗜️  Session Memory Compacted [%s]\n", mode)
	fmt.Println("-----------------------------------------------------------------")
	if res.CompactedTasks > 0 {
		fmt.Printf("📦 Tasks Archived        : %d tasks\n", res.CompactedTasks)
		fmt.Printf("📌 Tasks Retained Active : %d recent tasks\n", res.RetainedTasks)
	}
	if res.StateCompacted {
		fmt.Printf("📜 State Cleaned         : %d bytes removed from historical sections\n", res.StateBytesSaved)
	}
	fmt.Printf("📉 Size Reduction        : %d B ➔ %d B (-%d bytes)\n", res.OriginalBytes, res.CompactedBytes, res.OriginalBytes-res.CompactedBytes)
	fmt.Printf("⚡ Context Tokens Saved  : ~%d tokens\n", res.TokensSavedEst)
	if !*dryRun {
		if res.ArchiveFile != "" {
			fmt.Printf("📁 Tasks Archive         : %s\n", res.ArchiveFile)
			fmt.Println("✅ Active todo.md refreshed with clean rollup note.")
		}
		if res.StateArchiveFile != "" {
			fmt.Printf("📁 State Archive         : %s\n", res.StateArchiveFile)
			fmt.Println("✅ Active state.md pruned of stale historical logs.")
		}
	}
	fmt.Println("-----------------------------------------------------------------")
}

func runSessionWatch(info *platform.Info, args []string) {
	watchCmd := flag.NewFlagSet("session watch", flag.ExitOnError)
	once := watchCmd.Bool("once", false, "Exibe os passos recentes e encerra sem acompanhar em tempo real")
	notify := watchCmd.Bool("notify", true, "Emite notificação no SO quando o agente fizer uma pergunta")
	steps := watchCmd.Int("steps", 5, "Número de passos recentes para exibir inicialmente")
	tree := watchCmd.Bool("tree", false, "Exibe a árvore de subagentes e mensagens inter-agentes")
	_ = watchCmd.Parse(args)

	tInfo, err := watcher.FindActiveTranscript(info.GeminiDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro localizando transcrição: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📡 Streaming Antigravity Brain [%s]\n", tInfo.ConversationID)
	fmt.Printf("📄 Transcrição: %s\n", tInfo.Path)
	fmt.Println("-----------------------------------------------------------------")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts := watcher.WatchOptions{
		Follow:       !*once,
		NotifyOnWait: *notify,
		InitialSteps: *steps,
		OSName:       info.OS,
	}

	err = watcher.Stream(ctx, tInfo.Path, opts, func(evt *watcher.Event) {
		summary := evt.Summary()
		if summary != "" {
			fmt.Println(summary)
		}
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Erro no stream da transcrição: %v\n", err)
		os.Exit(1)
	}

	if *tree {
		fmt.Println("\n" + watcher.GetGlobalSubagentTree().FormatTree())
	}

	if !*once {
		fmt.Println("\n🛑 Stream encerrado.")
	}
}

func runDoctor(info *platform.Info, args []string) {
	docCmd := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonOut := docCmd.Bool("json", false, "Output doctor diagnostics as JSON")
	fixFlag := docCmd.Bool("fix", false, "Automatically repair missing session files and corrupted state")
	_ = docCmd.Parse(args)

	targetDir := "."
	if docCmd.NArg() > 0 {
		targetDir = docCmd.Arg(0)
	}

	if *fixFlag {
		res, err := doctor.Fix(targetDir, info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error auto-repairing environment: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Println("🩺 Antigravity Operator Self-Healing Doctor:")
		if len(res.Repaired) > 0 {
			fmt.Printf("   ✨ Reparados/Restaurados (%d):\n", len(res.Repaired))
			for _, r := range res.Repaired {
				fmt.Printf("      + %s\n", r)
			}
		}
		if len(res.Skipped) > 0 {
			fmt.Printf("   🛡️  Arquivos intactos preservados (%d):\n", len(res.Skipped))
			for _, s := range res.Skipped {
				fmt.Printf("      - %s\n", s)
			}
		}
		fmt.Println("✅ Auto-recuperação concluída com sucesso!")
		return
	}

	report := doctor.Run(info)

	if *jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("🔍 Antigravity Operator Doctor [OS: %s | Arch: %s]\n", info.OS, info.Arch)
	if info.HasDisplay {
		fmt.Println("🖥️  Display Server: Detected (Desktop GUI)")
	} else {
		fmt.Println("🖥️  Display Server: Not detected (Headless Mode Active)")
	}
	fmt.Println("-----------------------------------------------------------------")

	for _, chk := range report.Checks {
		var icon string
		switch chk.Status {
		case "OK":
			icon = "✅"
		case "WARN":
			icon = "⚠️ "
		case "FAIL":
			icon = "❌"
		default:
			icon = "ℹ️ "
		}
		fmt.Printf("%s %-28s : %s\n", icon, chk.Name, chk.Details)
	}
	fmt.Println("-----------------------------------------------------------------")
}

func runBrowser(info *platform.Info, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: agyo browser [start|status|stop]")
		os.Exit(1)
	}

	sub := args[0]
	switch sub {
	case "status":
		st := profile.CheckStatus(info.BrowserProfile)
		if st.IsRunning {
			pidInfo := ""
			if st.PID > 0 {
				pidInfo = fmt.Sprintf(" [PID: %d]", st.PID)
			}
			fmt.Printf("✅ Chrome DevTools ACTIVE on port %d (%s)%s\n", st.Port, st.Version, pidInfo)
			fmt.Printf("   Isolated profile: %s\n", st.ProfileDir)
		} else {
			fmt.Printf("⚠️  Chrome DevTools INACTIVE on port %d\n", st.Port)
			fmt.Printf("   To launch, run: agyo browser start\n")
		}
	case "start":
		browserCmd := flag.NewFlagSet("browser start", flag.ExitOnError)
		headless := browserCmd.Bool("headless", false, "Force headless mode even with display available")
		port := browserCmd.Int("port", profile.DefaultDebugPort, "Chrome remote debugging port")
		_ = browserCmd.Parse(args[1:])

		fmt.Printf("🚀 Launching isolated Chrome for Google Antigravity on port %d...\n", *port)
		err := profile.Start(info, profile.StartOptions{
			Port:          *port,
			ForceHeadless: *headless,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting browser: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Isolated Chrome active on port %d!\n", *port)
	case "stop":
		fmt.Println("🛑 Terminating isolated Chrome instance...")
		err := profile.Stop(info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning terminating browser: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Isolated Chrome terminated successfully.")

	case "tabs":
		tabs, err := profile.ListTabs(profile.DefaultDebugPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing browser tabs: %v\n", err)
			os.Exit(1)
		}
		if len(tabs) == 0 {
			fmt.Println("ℹ️  No open page tabs found in Chrome.")
			return
		}
		fmt.Printf("🌐 Open Chrome Tabs (%d active):\n", len(tabs))
		fmt.Println("-----------------------------------------------------------------")
		for _, t := range tabs {
			idPrefix := t.ID
			if len(idPrefix) > 8 {
				idPrefix = idPrefix[:8]
			}
			fmt.Printf("[%s] %-35s : %s\n", idPrefix, t.Title, t.URL)
		}
		fmt.Println("-----------------------------------------------------------------")

	case "open":
		if len(args) < 2 {
			fmt.Println("Usage: agyo browser open <url>")
			os.Exit(1)
		}
		targetURL := args[1]
		tab, err := profile.OpenTab(profile.DefaultDebugPort, targetURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening tab: %v\n", err)
			os.Exit(1)
		}
		idPrefix := tab.ID
		if len(idPrefix) > 8 {
			idPrefix = idPrefix[:8]
		}
		fmt.Printf("✅ Tab opened [%s]: %s\n", idPrefix, targetURL)

	case "close":
		if len(args) < 2 {
			fmt.Println("Usage: agyo browser close <tab-id>")
			os.Exit(1)
		}
		tabID := args[1]
		if err := profile.CloseTab(profile.DefaultDebugPort, tabID); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing tab: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Tab [%s] closed successfully.\n", tabID)

	case "eval":
		if len(args) < 2 {
			fmt.Println("Usage: agyo browser eval \"<javascript>\"")
			os.Exit(1)
		}
		expr := args[1]
		val, err := profile.Eval(profile.DefaultDebugPort, expr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error evaluating JS via CDP: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(val)

	case "shot", "screenshot":
		dest := "screenshot.png"
		if len(args) > 1 {
			dest = args[1]
		}
		if err := profile.Screenshot(profile.DefaultDebugPort, dest); err != nil {
			fmt.Fprintf(os.Stderr, "Error capturing screenshot: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📸 Screenshot saved successfully to: %s\n", dest)

	default:
		fmt.Fprintf(os.Stderr, "Unknown browser subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func runSync(info *platform.Info) {
	fmt.Println("🔄 Synchronizing rules and manifests into Google Antigravity...")
	res, err := installer.Sync(info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during synchronization: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Rules synchronized at: %s\n", res.RulesPath)
	if res.MCPPath != "" {
		fmt.Printf("✅ MCP manifest prepared at: %s\n", res.MCPPath)
	}
	if res.SkillsPath != "" {
		fmt.Printf("✅ Antigravity skill installed at: %s\n", res.SkillsPath)
	}
}

func runHook(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: agyo hook [install|uninstall] [dir]")
		os.Exit(1)
	}
	sub := args[0]
	targetDir := "."
	if len(args) > 1 {
		targetDir = args[1]
	}
	switch sub {
	case "install":
		path, err := hook.Install(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error installing git hook: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Pre-commit session hook installed at: %s\n", path)
	case "uninstall":
		if err := hook.Uninstall(targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error uninstalling git hook: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Pre-commit session hook uninstalled successfully.")
	default:
		fmt.Fprintf(os.Stderr, "Unknown hook subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func runSessionExport(info *platform.Info, args []string) {
	exportCmd := flag.NewFlagSet("session export", flag.ExitOnError)
	format := exportCmd.String("format", "markdown", "Formato de exportação: markdown ou html")
	out := exportCmd.String("out", "", "Caminho do arquivo de saída (opcional, padrão imprime na tela)")
	_ = exportCmd.Parse(args)

	targetDir := "."
	if exportCmd.NArg() > 0 {
		targetDir = exportCmd.Arg(0)
	}

	transcriptPath := ""
	if tInfo, err := watcher.FindActiveTranscript(info.GeminiDir); err == nil {
		transcriptPath = tInfo.Path
	}

	opts := exporter.ExportOptions{
		Format:         *format,
		TargetDir:      targetDir,
		OutputPath:     *out,
		TranscriptPath: transcriptPath,
	}

	res, err := exporter.Export(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao exportar sessão: %v\n", err)
		os.Exit(1)
	}

	if *out != "" {
		fmt.Printf("✅ Relatório de sessão exportado com sucesso para: %s\n", *out)
	} else {
		fmt.Println(res)
	}
}

func runDashboard(info *platform.Info, args []string) {
	dashCmd := flag.NewFlagSet("dashboard", flag.ExitOnError)
	port := dashCmd.Int("port", 8080, "Porta do servidor HTTP do dashboard")
	open := dashCmd.Bool("open", true, "Abre automaticamente o navegador padrão")
	_ = dashCmd.Parse(args)

	targetDir := "."
	if dashCmd.NArg() > 0 {
		targetDir = dashCmd.Arg(0)
	}

	cfg := dashboard.Config{
		Port:         *port,
		TargetDir:    targetDir,
		PlatformInfo: info,
		OpenBrowser:  *open,
	}

	srv, err := dashboard.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao iniciar dashboard: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Println("\n🛑 Encerrando dashboard...")
		shutdownCtx, sCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer sCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Servidor encerrado: %v\n", err)
	}
}

func runCompletion(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: agyo completion [bash|zsh|fish]")
		fmt.Println("\nExample for zsh:")
		fmt.Println("  source <(agyo completion zsh)")
		fmt.Println("\nExample for bash:")
		fmt.Println("  source <(agyo completion bash)")
		os.Exit(1)
	}

	shell := args[0]
	if err := completion.Generate(shell, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCheckpoint(args []string) {
	chkCmd := flag.NewFlagSet("checkpoint", flag.ExitOnError)
	desc := chkCmd.String("desc", "", "Optional description for the checkpoint")
	listFlag := chkCmd.Bool("list", false, "List all saved checkpoints")
	jsonOut := chkCmd.Bool("json", false, "Output checkpoints list as JSON")
	_ = chkCmd.Parse(args)

	targetDir := "."
	name := ""
	if chkCmd.NArg() == 1 {
		if fi, err := os.Stat(chkCmd.Arg(0)); err == nil && fi.IsDir() {
			targetDir = chkCmd.Arg(0)
		} else {
			name = chkCmd.Arg(0)
		}
	} else if chkCmd.NArg() > 1 {
		name = chkCmd.Arg(0)
		targetDir = chkCmd.Arg(1)
	}

	if *listFlag {
		list, err := checkpoint.List(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing checkpoints: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			data, _ := json.MarshalIndent(list, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(list) == 0 {
			fmt.Println("📍 Nenhum checkpoint encontrado em .agents/session/checkpoints.json")
			return
		}
		fmt.Printf("🛡️  Checkpoints Salvos (%d encontrados):\n", len(list))
		fmt.Println("-----------------------------------------------------------------")
		for _, c := range list {
			fmt.Printf("🔖 [%s] %s\n", c.ID, c.Name)
			fmt.Printf("   📅 Criado em   : %s\n", c.Timestamp)
			fmt.Printf("   🌿 Branch/Commit: %s (%s)\n", c.Branch, c.CommitSHA)
			if len(c.DirtyFiles) > 0 {
				fmt.Printf("   📝 Dirty files  : %d arquivos rastreados\n", len(c.DirtyFiles))
			}
			if c.Description != "" {
				fmt.Printf("   💬 Descrição    : %s\n", c.Description)
			}
			fmt.Println()
		}
		fmt.Println("💡 Para voltar ao estado de um checkpoint: agyo rollback <checkpoint-id>")
		return
	}

	chk, err := checkpoint.Create(targetDir, name, *desc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating checkpoint: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🛡️  Checkpoint criado com sucesso!\n")
	fmt.Printf("   ID        : %s\n", chk.ID)
	fmt.Printf("   Nome      : %s\n", chk.Name)
	fmt.Printf("   Branch    : %s (%s)\n", chk.Branch, chk.CommitSHA)
	if len(chk.DirtyFiles) > 0 {
		fmt.Printf("   Modificados: %d arquivos preservados no stash commit (%s)\n", len(chk.DirtyFiles), chk.StashSHA)
	} else {
		fmt.Printf("   Modificados: Working tree limpa (clean tree)\n")
	}
	fmt.Println("💡 Em caso de falha ou refatoração indesejada: agyo rollback")
}

func runRollback(args []string) {
	targetID := ""
	targetDir := "."

	if len(args) == 1 {
		// Se for um diretório existente, usa como targetDir, senão é targetID
		if fi, err := os.Stat(args[0]); err == nil && fi.IsDir() {
			targetDir = args[0]
		} else {
			targetID = args[0]
		}
	} else if len(args) > 1 {
		targetID = args[0]
		targetDir = args[1]
	}

	res, err := checkpoint.Rollback(targetDir, targetID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rolling back checkpoint: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("⏪ Workspace restaurado com sucesso para o checkpoint:\n")
	fmt.Printf("   ID        : %s\n", res.ID)
	fmt.Printf("   Nome      : %s\n", res.Name)
	fmt.Printf("   Timestamp : %s\n", res.Timestamp)
	fmt.Printf("   Branch    : %s (%s)\n", res.Branch, res.CommitSHA)
	if len(res.DirtyFiles) > 0 {
		fmt.Printf("   Arquivos  : %d modificações restauradas na working tree\n", len(res.DirtyFiles))
	}
}
