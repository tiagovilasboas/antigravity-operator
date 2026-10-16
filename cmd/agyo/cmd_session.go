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

	"github.com/tiagovilasboas/antigravity-operator/internal/dashboard"
	"github.com/tiagovilasboas/antigravity-operator/internal/exporter"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/session"
	"github.com/tiagovilasboas/antigravity-operator/internal/watcher"
)

func runInit(args []string) {
	if hasHelp(args) {
		fmt.Println("Usage: agyo init [-force] [dir]")
		fmt.Println("Scaffold operational memory (.agents/session/) in the target project.")
		return
	}
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	force := initCmd.Bool("force", false, "Overwrite existing session files")
	_ = initCmd.Parse(args)

	targetDir := "."
	if initCmd.NArg() > 0 {
		targetDir = initCmd.Arg(0)
	}

	res, err := session.Init(targetDir, *force)
	if err != nil {
		runtimeError(err, "")
		return
	}

	fmt.Printf("✅ Session memory initialized at: %s\n", res.SessionDir)
	for _, c := range res.Created {
		fmt.Printf("   + Created: %s\n", c)
	}
	for _, s := range res.Skipped {
		fmt.Printf("   - Kept (already exists): %s\n", s)
	}
}

func sessionUsageString() string {
	return `Usage: agyo session <subcommand> [options]

Subcommands:
  status [--json] [dir]           Display active session objective, status, and task completion metrics
  compact [--threshold N] [dir]   Archive completed tasks and rollup active todo.md to avoid context bloat
  archive [dir]                   Archive completed session to historical log and reset templates
  list [--json] [dir]             List all archived historical sessions
  restore [file] [dir]            Restore a past archived session into active memory with automatic backup
  watch, tail [dir]               Stream active agent reasoning and desktop notifications in real time
  export [--format md|html] [dir] Export consolidated session report in markdown or HTML format

Examples:
  agyo session status
  agyo session status --json
  agyo session compact --threshold 5
  agyo session list --json

Run 'agyo session <subcommand> --help' for details on each subcommand.`
}

func printSessionUsage() {
	fmt.Println(sessionUsageString())
}

func runSession(info *platform.Info, args []string) {
	if len(args) == 0 {
		printSessionUsage()
		exitFunc(ExitUsageError)
		return
	}

	sub := args[0]
	if isHelp(sub) {
		printSessionUsage()
		return
	}

	targetDir := "."

	switch sub {
	case "status":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session status [--json] [dir]")
			fmt.Println("Display active session objective, status, and task completion metrics.")
			return
		}
		statusCmd := flag.NewFlagSet("session status", flag.ExitOnError)
		jsonOut := statusCmd.Bool("json", false, "Output session status as JSON")
		_ = statusCmd.Parse(args[1:])
		if statusCmd.NArg() > 0 {
			targetDir = statusCmd.Arg(0)
		}

		sum, err := session.GetSummary(targetDir)
		if err != nil {
			runtimeError(err, "Run 'agyo init' to initialize session memory in this directory")
			return
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
		if len(sum.Warnings) > 0 {
			fmt.Println()
			fmt.Println("⚠️  Format warning in state.md:")
			for _, w := range sum.Warnings {
				fmt.Printf("   • %s\n", w)
			}
		}
		fmt.Println("-----------------------------------------------------------------")

	case "compact":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session compact [--threshold N] [--keep N] [--dry-run] [dir]")
			fmt.Println("Archive completed tasks and rollup active todo.md to avoid context bloat.")
			return
		}
		runSessionCompact(args[1:])

	case "archive":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session archive [dir]")
			fmt.Println("Archive completed session to historical log and reset templates.")
			return
		}
		if len(args) > 1 {
			targetDir = args[1]
		}
		archiveFile, err := session.Archive(targetDir)
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Printf("📦 Session archived successfully to:\n   %s\n", archiveFile)
		fmt.Println("✅ State and tasks reset with clean templates for the next task!")

	case "list":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session list [--json] [dir]")
			fmt.Println("List all archived historical sessions.")
			return
		}
		listCmd := flag.NewFlagSet("session list", flag.ExitOnError)
		jsonOut := listCmd.Bool("json", false, "Output archives list as JSON")
		_ = listCmd.Parse(args[1:])
		if listCmd.NArg() > 0 {
			targetDir = listCmd.Arg(0)
		}

		archives, err := session.ListArchives(targetDir)
		if err != nil {
			runtimeError(err, "")
			return
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
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session restore [archive-file] [dir]")
			fmt.Println("Restore a past archived session into active memory with automatic backup.")
			return
		}
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
			runtimeError(err, "Run 'agyo session list' to inspect available session archives")
			return
		}
		fmt.Printf("♻️  Sessão restaurada com sucesso a partir de:\n   %s\n", res.RestoredFile)
		if res.BackupFile != "" {
			fmt.Printf("🛡️  Backup preventivo da sessão anterior salvo em:\n   %s\n", res.BackupFile)
		}
		fmt.Printf("🎯 Objetivo restaurado : %s\n", res.Objective)
		fmt.Printf("📋 Tarefas restauradas  : %d\n", res.TasksCount)

	case "watch", "tail":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session watch [dir]")
			fmt.Println("Stream active agent reasoning and desktop notifications in real time.")
			return
		}
		runSessionWatch(info, args[1:])

	case "export":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo session export [--format md|html] [--out file] [dir]")
			fmt.Println("Export consolidated session report in markdown or HTML format.")
			return
		}
		runSessionExport(info, args[1:])

	default:
		usageError("unknown session subcommand %q\nRun 'agyo session --help' for usage.", sub)
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
		runtimeError(err, "")
		return
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

// homeDir returns the user's home for transcript discovery.
func homeDir(info *platform.Info) string {
	if info.HomeDir != "" {
		return info.HomeDir
	}
	home, _ := os.UserHomeDir()
	return home
}

func runSessionWatch(info *platform.Info, args []string) {
	watchCmd := flag.NewFlagSet("session watch", flag.ExitOnError)
	once := watchCmd.Bool("once", false, "Exibe os passos recentes e encerra sem acompanhar em tempo real")
	notify := watchCmd.Bool("notify", true, "Emite notificação no SO quando o agente fizer uma pergunta")
	steps := watchCmd.Int("steps", 5, "Número de passos recentes para exibir inicialmente")
	tree := watchCmd.Bool("tree", false, "Exibe a árvore de subagentes e mensagens inter-agentes")
	_ = watchCmd.Parse(args)

	tInfo, err := watcher.FindLatestTranscript(homeDir(info))
	if err != nil {
		runtimeError(err, "")
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
		runtimeError(err, "")
	}

	if *tree {
		fmt.Println("\n" + watcher.GetGlobalSubagentTree().FormatTree())
	}

	if !*once {
		fmt.Println("\n🛑 Stream encerrado.")
	}
}

func runSessionExport(info *platform.Info, args []string) {
	if hasHelp(args) {
		fmt.Println("Usage: agyo session export [--format md|html] [--out file] [dir]")
		fmt.Println("Export consolidated session report in markdown or HTML format.")
		return
	}
	exportCmd := flag.NewFlagSet("session export", flag.ExitOnError)
	format := exportCmd.String("format", "markdown", "Formato de exportação: markdown ou html")
	out := exportCmd.String("out", "", "Caminho do arquivo de saída (opcional, padrão imprime na tela)")
	_ = exportCmd.Parse(args)

	targetDir := "."
	if exportCmd.NArg() > 0 {
		targetDir = exportCmd.Arg(0)
	}

	transcriptPath := ""
	if tInfo, err := watcher.FindLatestTranscript(homeDir(info)); err == nil {
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
		runtimeError(err, "")
		return
	}

	if *out != "" {
		fmt.Printf("✅ Relatório de sessão exportado com sucesso para: %s\n", *out)
	} else {
		fmt.Println(res)
	}
}

func runDashboard(info *platform.Info, args []string) {
	if hasHelp(args) {
		fmt.Println("Usage: agyo dashboard [-port <p>] [-open] [dir]")
		fmt.Println("Launch local web dashboard for live monitoring and DevTools inspection.")
		return
	}
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
		runtimeError(err, "")
		return
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
