package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

// Version is set at build time from the git tag:
//
//	go build -ldflags "-X main.Version=0.4.5" ./cmd/agyo
//
// Builds without the flag report "dev".
var Version = "dev"

// Standard CLI exit codes
const (
	ExitSuccess      = 0
	ExitRuntimeError = 1
	ExitUsageError   = 2
)

var exitFunc = os.Exit
var isTerminalFunc = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func isHelp(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

func hasHelp(args []string) bool {
	for _, a := range args {
		if isHelp(a) {
			return true
		}
	}
	return false
}

func hasJSON(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "-json" {
			return true
		}
	}
	return false
}

// permuteArgs reorders arguments so that flags (and their values) appear before
// positional arguments. This prevents Go stdlib flag.FlagSet from stopping flag parsing
// at the first positional argument.
func permuteArgs(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			trimmed := strings.TrimLeft(arg, "-")
			if strings.Contains(trimmed, "=") {
				flags = append(flags, arg)
				continue
			}
			if trimmed == "help" || trimmed == "h" || (boolFlags != nil && boolFlags[trimmed]) {
				flags = append(flags, arg)
				continue
			}
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}

	return append(flags, positional...)
}

func usageError(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
	exitFunc(ExitUsageError)
}

func runtimeError(err error, hint string) {
	msg := err.Error()
	if strings.Contains(msg, "127.0.0.1:9222") || strings.Contains(msg, "connection refused") {
		fmt.Fprintf(os.Stderr, "Error: Chrome DevTools is not reachable on port 9222\n→ Start isolated Chrome with: agyo browser start\n")
		exitFunc(ExitRuntimeError)
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	if hint != "" {
		fmt.Fprintf(os.Stderr, "→ %s\n", hint)
	}
	exitFunc(ExitRuntimeError)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		exitFunc(ExitUsageError)
	}

	info, err := platform.Detect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting platform: %v\n", err)
		exitFunc(ExitRuntimeError)
	}

	command := os.Args[1]
	if isHelp(command) {
		printUsage()
		return
	}

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
	case "computer":
		runComputer(info, os.Args[2:])
	case "notebook", "notebooklm", "nblm":
		runNotebook(info, os.Args[2:])
	case "sync":
		runSync(info, os.Args[2:])
	case "hook":
		runHook(os.Args[2:])
	case "google", "gsuite", "workspace":
		runGoogle(info, os.Args[2:])
	case "study":
		runStudy(info, os.Args[2:])
	case "completion":
		runCompletion(os.Args[2:])
	case "about":
		printAbout()
	case "version", "-v", "--version":
		if hasJSON(os.Args[2:]) {
			data, _ := json.MarshalIndent(map[string]string{
				"version": Version,
				"os":      info.OS,
				"arch":    info.Arch,
			}, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Printf("agyo (Antigravity Operator) v%s [%s/%s]\n", Version, info.OS, info.Arch)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", command)
		if suggestion := suggestCommand(command); suggestion != "" {
			fmt.Fprintf(os.Stderr, "Did you mean 'agyo %s'?\n", suggestion)
		}
		fmt.Fprintf(os.Stderr, "Run 'agyo --help' for usage.\n")
		exitFunc(ExitUsageError)
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
  browser start       Launch isolated Chrome instance with remote debugging flags (--profile supported)
  browser status      Inspect DevTools port (9222) readiness and Chrome process PID
  browser profiles    Discover and classify Chrome profiles (personal, workspace, ghost)
  browser resolve <u> Resolve the best authenticated profile and authuser URL for a domain/URL
  browser stop        Gracefully terminate isolated Chrome process (SIGTERM)
  browser tabs        List all open tabs and target IDs in the isolated Chrome
  browser open <url>  Open or reuse a tab at the given URL in the isolated Chrome
  browser close <id>  Close a specific tab by ID
  browser eval "<js>" Evaluate JavaScript expression in the active tab (pure Go CDP)
  browser shot [file] Capture PNG screenshot of active tab without external libraries
  computer inspect    Inspect active window and extract semantic Accessibility Tree (--json)
  computer shot [f]   Capture compressed, token-optimized screenshot (JPEG 1280px by default)
  computer click <r>  Click element by ref (e.g. ref_1) or coordinates (x y) (--double, --right)
  computer type "<t>" Type text string into focused element
  computer key "<k>"  Press key or shortcut (e.g. return, esc, cmd+space, cmd+c)
  computer batch '<j>' Execute macro sequence of actions in one call
  computer mcp        Serve stdio Model Context Protocol (MCP) server for agents
  notebooklm status   Inspect Google NotebookLM connection and auth readiness
  notebooklm open     Launch isolated Chrome directly into Google NotebookLM
  notebooklm list     List all notebooks available in the active Google account
  notebooklm ask <id> Ask a grounded query to a NotebookLM notebook
  notebooklm push <id> Push markdown or text file as a source to a notebook
  notebooklm mcp      Serve stdio Model Context Protocol (MCP) server for agents
  google auth         Authenticate with Google Workspace via local loopback OAuth
  google calendar     List or schedule Google Calendar events (supports --json)
  google docs         Read or create Google Docs documents (supports --json)
  study ingest <url>  Absorb YouTube/web sources and generate Feynman guides + flashcards
  study exam-prep     Calculate reverse study schedule and sync to todo.md / Google Calendar
  study quiz          Interactive Feynman checkpoint with instant AI tutor feedback
  sync [--update-mcp] Synchronize canonical rules and MCP manifests to Google Antigravity
                      (--update-mcp backs up and rewrites an outdated MCP manifest)
  hook install [dir]  Install git pre-commit hook to safeguard session continuity
  hook uninstall [dir] Remove agyo git pre-commit hook
  completion [shell]  Generate shell autocompletion script (bash, zsh, fish)
  about               Display manifesto and tribute to the community & Google AI Pro
  version             Print version and system architecture`)
}

func printAbout() {
	fmt.Println(`================================================================================
  ANTIGRAVITY OPERATOR (agyo) - SESSION RUNTIME & OUTER HARNESS
================================================================================

This project is an open engineering tribute to the developer community, students,
and researchers worldwide, and a special thank you to Google for the transformative
student access program through Google AI Pro.

Mission:
Connect the deep research intelligence of Google NotebookLM (the Brain) to the autonomous
execution power of Google Antigravity (the Hands), governed by a deterministic Outer
Harness with filesystem memory (.agents/session/) and seamless parity across macOS and Linux -
empowering every student and engineer to build grounded, verified software without token
waste, hallucinations, or runtime friction.

Built with care, precision, and canonical software engineering (Martin Fowler Outer Harness).
================================================================================`)
}
