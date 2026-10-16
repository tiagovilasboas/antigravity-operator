package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/completion"
	"github.com/tiagovilasboas/antigravity-operator/internal/doctor"
	"github.com/tiagovilasboas/antigravity-operator/internal/hook"
	"github.com/tiagovilasboas/antigravity-operator/internal/installer"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func runDoctor(info *platform.Info, args []string) {
	if hasHelp(args) {
		fmt.Println("Usage: agyo doctor [--json] [--fix] [dir]")
		fmt.Println("Audit host readiness, platform tools, and auto-repair session memory.")
		return
	}
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
			runtimeError(err, "")
			return
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

func runSync(info *platform.Info, args []string) {
	if hasHelp(args) {
		fmt.Println("Usage: agyo sync [--update-mcp]")
		fmt.Println("Synchronize canonical rules, skills, and MCP manifests into Google Antigravity.")
		return
	}
	syncCmd := flag.NewFlagSet("sync", flag.ExitOnError)
	updateMCP := syncCmd.Bool("update-mcp", false, "Back up the installed MCP manifest and rewrite it from the built-in template")
	_ = syncCmd.Parse(args)

	fmt.Println("🔄 Synchronizing rules and manifests into Google Antigravity...")
	res, err := installer.Sync(info)
	if err != nil {
		runtimeError(err, "")
		return
	}
	fmt.Printf("✅ Rules synchronized at: %s\n", res.RulesPath)
	if *updateMCP {
		backup, changed, err := installer.UpdateMCP(info, time.Now())
		if err != nil {
			runtimeError(err, "")
			return
		}
		if backup != "" {
			fmt.Printf("🗂️  Previous MCP manifest backed up to: %s\n", backup)
		}
		if changed {
			fmt.Printf("✅ MCP manifest rewritten from the built-in template: %s\n", installer.MCPPath(info))
		} else {
			fmt.Printf("✅ MCP manifest already matches the built-in template: %s\n", installer.MCPPath(info))
		}
	} else if res.MCPPath != "" {
		fmt.Printf("✅ MCP manifest prepared at: %s\n", res.MCPPath)
		if installer.CheckMCP(info).NeedsUpdate() {
			fmt.Println("⚠️  Existing MCP manifest differs from the built-in template (kept as is). Run 'agyo sync --update-mcp' to back it up and rewrite it.")
		}
	}
	if res.SkillsPath != "" {
		fmt.Printf("✅ Antigravity skill installed at: %s\n", res.SkillsPath)
	}
	for _, b := range res.Backups {
		fmt.Printf("🗂️  Previous file backed up to: %s\n", b)
	}
	for _, s := range res.Skipped {
		fmt.Printf("ℹ️  Preserved external symlink: %s\n", s)
	}
}

func printHookUsage() {
	fmt.Println(`Usage: agyo hook <subcommand> [arguments]

Subcommands:
  install [dir]                   Install git pre-commit hook to safeguard session continuity
  uninstall [dir]                 Remove agyo git pre-commit hook
  antigravity install [--scope user|workspace] [--dry-run]
                                  Register agyo lifecycle hooks into Google Antigravity (hooks.json)
  antigravity uninstall [--scope user|workspace]
                                  Unregister agyo lifecycle hooks from Google Antigravity
  antigravity status [--scope user|workspace]
                                  Check if agyo lifecycle hooks are active in hooks.json
  run <PreInvocation|PreToolUse|PostToolUse|Stop>
                                  Execute lifecycle hook event handler (stdin JSON -> stdout JSON)

Examples:
  agyo hook install
  agyo hook antigravity install
  agyo hook antigravity status

Run 'agyo hook <subcommand> --help' for details on each subcommand.`)
}

func runHook(args []string) {
	if len(args) == 0 {
		printHookUsage()
		exitFunc(ExitUsageError)
		return
	}
	sub := args[0]
	if isHelp(sub) {
		printHookUsage()
		return
	}

	switch sub {
	case "install":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo hook install [dir]")
			fmt.Println("Install git pre-commit hook to safeguard session continuity.")
			return
		}
		targetDir := "."
		if len(args) > 1 {
			targetDir = args[1]
		}
		path, err := hook.Install(targetDir)
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Printf("✅ Pre-commit session hook installed at: %s\n", path)

	case "uninstall":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo hook uninstall [dir]")
			fmt.Println("Remove agyo git pre-commit hook.")
			return
		}
		targetDir := "."
		if len(args) > 1 {
			targetDir = args[1]
		}
		if err := hook.Uninstall(targetDir); err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println("✅ Pre-commit session hook uninstalled successfully.")

	case "antigravity":
		if len(args) < 2 || isHelp(args[1]) {
			fmt.Println(`Usage: agyo hook antigravity <subcommand> [options]

Subcommands:
  install [--scope user|workspace] [--dry-run] [dir]
  uninstall [--scope user|workspace] [dir]
  status [--scope user|workspace] [dir]`)
			if len(args) < 2 {
				exitFunc(ExitUsageError)
			}
			return
		}
		action := args[1]
		fs := flag.NewFlagSet("hook antigravity "+action, flag.ExitOnError)
		scope := fs.String("scope", "user", "Scope for hooks.json (user or workspace)")
		dryRun := fs.Bool("dry-run", false, "Preview changes without writing to disk")
		_ = fs.Parse(args[2:])

		workspace := "."
		if fs.NArg() > 0 {
			workspace = fs.Arg(0)
		}

		switch action {
		case "install":
			path, content, err := hook.InstallAntigravityHook(*scope, workspace, *dryRun)
			if err != nil {
				runtimeError(err, "")
				return
			}
			if *dryRun {
				fmt.Printf("🔍 [Dry Run] Target file: %s\n", path)
				fmt.Println(content)
				return
			}
			fmt.Printf("✅ Antigravity lifecycle hook (%s) registered at: %s\n", hook.AgyoHookName, path)
		case "uninstall":
			path, err := hook.UninstallAntigravityHook(*scope, workspace)
			if err != nil {
				runtimeError(err, "")
				return
			}
			fmt.Printf("✅ Antigravity lifecycle hook (%s) removed from: %s\n", hook.AgyoHookName, path)
		case "status":
			active, path, err := hook.AntigravityHookStatus(*scope, workspace)
			if err != nil {
				runtimeError(err, "")
				return
			}
			if active {
				fmt.Printf("✅ Antigravity lifecycle hook (%s) is ACTIVE at: %s\n", hook.AgyoHookName, path)
			} else {
				fmt.Printf("⚠️  Antigravity lifecycle hook (%s) is NOT active at: %s\n", hook.AgyoHookName, path)
				fmt.Println("→ Install with: agyo hook antigravity install")
			}
		default:
			usageError("unknown antigravity hook action %q\nRun 'agyo hook antigravity --help' for usage.", action)
			return
		}

	case "run":
		if len(args) < 2 || isHelp(args[1]) {
			fmt.Println("Usage: agyo hook run <PreInvocation|PreToolUse|PostToolUse|Stop>")
			if len(args) < 2 {
				exitFunc(ExitUsageError)
			}
			return
		}
		event := args[1]
		_ = hook.DispatchHookEvent(context.Background(), event, os.Stdin, os.Stdout)
		return

	default:
		usageError("unknown hook subcommand %q\nRun 'agyo hook --help' for usage.", sub)
	}
}

func runCompletion(args []string) {
	if len(args) == 0 || hasHelp(args) {
		fmt.Println("Usage: agyo completion [bash|zsh|fish]")
		fmt.Println("\nExample for zsh:")
		fmt.Println("  source <(agyo completion zsh)")
		fmt.Println("\nExample for bash:")
		fmt.Println("  source <(agyo completion bash)")
		if hasHelp(args) {
			return
		}
		exitFunc(ExitUsageError)
		return
	}

	shell := args[0]
	if err := completion.Generate(shell, os.Stdout); err != nil {
		usageError("%v", err)
	}
}
