package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/notebook"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

func runNotebook(info *platform.Info, args []string) {
	if len(args) == 0 {
		printNotebookUsage()
		exitFunc(ExitUsageError)
		return
	}

	sub := args[0]
	if isHelp(sub) {
		printNotebookUsage()
		return
	}

	switch sub {
	case "status":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook status [--json]")
			fmt.Println("Check connection and authentication status with Google NotebookLM.")
			return
		}
		st := notebook.CheckSession(info, profile.DefaultDebugPort)
		if hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(data))
			return
		}
		if st.IsLoggedIn {
			fmt.Printf("✅ Google NotebookLM CONNECTED and AUTHENTICATED\n")
			fmt.Printf("   Active URL: %s\n", st.ActiveURL)
			if st.TabID != "" {
				fmt.Printf("   Tab ID: %s\n", st.TabID)
			}
		} else if st.HasTab {
			fmt.Printf("⚠️  Google NotebookLM tab open, but awaiting Google login\n")
			fmt.Printf("   URL: %s\n", st.ActiveURL)
			fmt.Printf("   Please complete login in the Chrome window.\n")
		} else if st.ChromeRunning {
			fmt.Printf("ℹ️  Chrome DevTools is running, but no NotebookLM tab is open.\n")
			fmt.Printf("   To open, run: agyo notebook open\n")
		} else {
			fmt.Printf("⚠️  Chrome DevTools is inactive.\n")
			fmt.Printf("   To start and open NotebookLM, run: agyo notebook open\n")
		}

	case "open", "login":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook open")
			fmt.Println("Launch isolated Chrome and open Google NotebookLM.")
			return
		}
		fmt.Println("🚀 Opening Google NotebookLM in isolated Chrome...")
		tab, err := notebook.OpenSession(info, profile.DefaultDebugPort)
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Printf("✅ NotebookLM tab ready at: %s\n", tab.URL)
		fmt.Println("   If needed, log in with your Google account. Sessions are preserved in your isolated profile.")

	case "list":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook list [--json]")
			fmt.Println("List all notebooks available in your Google account.")
			return
		}
		if !hasJSON(args[1:]) {
			fmt.Println("🔍 Fetching notebooks from Google NotebookLM...")
		}
		notebooks, err := notebook.ListNotebooks(info, profile.DefaultDebugPort)
		if err != nil {
			runtimeError(err, "")
			return
		}
		if hasJSON(args[1:]) {
			if notebooks == nil {
				notebooks = []notebook.Notebook{}
			}
			data, _ := json.MarshalIndent(notebooks, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(notebooks) == 0 {
			fmt.Println("ℹ️  No notebooks found on current page. Ensure you are logged in and on the NotebookLM homepage.")
			return
		}
		fmt.Printf("📚 Found %d Notebook(s):\n", len(notebooks))
		fmt.Println("-----------------------------------------------------------------")
		for _, nb := range notebooks {
			sourceInfo := ""
			if nb.SourceCount > 0 {
				sourceInfo = fmt.Sprintf(" (%d fontes)", nb.SourceCount)
			}
			fmt.Printf("[%s] %s%s\n    URL: %s\n", nb.ID, nb.Title, sourceInfo, nb.URL)
		}
		fmt.Println("-----------------------------------------------------------------")

	case "ask":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook ask <notebook-id> \"<query>\"")
			fmt.Println("Ask a grounded question to a NotebookLM notebook.")
			return
		}
		if len(args) < 3 {
			usageError("missing notebook ID or query\nUsage: agyo notebook ask <notebook-id> \"<query>\"")
			return
		}
		notebookID := args[1]
		query := strings.Join(args[2:], " ")
		fmt.Printf("❓ Submitting query to notebook [%s]...\n", notebookID)
		res, err := notebook.AskNotebook(info, profile.DefaultDebugPort, notebookID, query)
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println("\n💬 Answer:")
		fmt.Println(res.Answer)
		if len(res.Citations) > 0 {
			fmt.Println("\n📌 Citations / Sources:")
			for _, c := range res.Citations {
				fmt.Printf(" - %s\n", c)
			}
		}

	case "push":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook push <notebook-id> <file-or-note>")
			fmt.Println("Push markdown note or text file content into a notebook.")
			return
		}
		if len(args) < 3 {
			usageError("missing notebook ID or file path\nUsage: agyo notebook push <notebook-id> <file-or-note>")
			return
		}
		notebookID := args[1]
		filePath := args[2]

		contentBytes, err := os.ReadFile(filePath)
		var content string
		var title string
		if err == nil {
			content = string(contentBytes)
			title = filepath.Base(filePath)
		} else {
			content = strings.Join(args[2:], " ")
			title = "Note"
		}

		fmt.Printf("📤 Pushing note [%s] to notebook [%s]...\n", title, notebookID)
		if err := notebook.PushSource(info, profile.DefaultDebugPort, notebookID, title, content); err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println("✅ Note successfully submitted to notebook.")

	case "add-source", "source":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook add-source <notebook-id> <url>")
			fmt.Println("Add a YouTube or Web URL directly as a source to a notebook.")
			return
		}
		if len(args) < 3 {
			usageError("missing notebook ID or source URL\nUsage: agyo notebook add-source <notebook-id> <url>")
			return
		}
		notebookID := args[1]
		sourceURL := args[2]
		fmt.Printf("📥 Ingesting URL source into notebook [%s]...\n", notebookID)
		if err := notebook.AddSourceURL(info, profile.DefaultDebugPort, notebookID, sourceURL); err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println("✅ Source URL successfully submitted to Google NotebookLM.")

	case "mcp":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo notebook mcp")
			fmt.Println("Run stdio Model Context Protocol (MCP) server for agents.")
			return
		}
		if err := notebook.ServeMCP(info, os.Stdin, os.Stdout); err != nil {
			runtimeError(err, "")
			return
		}

	default:
		usageError("unknown notebook subcommand %q\nRun 'agyo notebook --help' for usage.", sub)
	}
}

func printNotebookUsage() {
	fmt.Println(`Usage: agyo notebooklm <subcommand> [options] (aliases: agyo notebook, agyo nblm)

Subcommands:
  status [--json]      Check connection and authentication status with Google NotebookLM
  open, login          Launch isolated Chrome and open Google NotebookLM
  list [--json]        List all notebooks available in your Google account
  ask <id> "<query>"   Ask a grounded question to a NotebookLM notebook
  push <id> <file>     Push markdown note/file content into a notebook
  add-source <id> <url> Add a YouTube or Web URL directly as a source to a notebook
  mcp                  Run stdio Model Context Protocol (MCP) server for agents

Examples:
  agyo notebook status
  agyo notebook list --json
  agyo notebook ask <id> "Quais os principais conceitos desse material?"
  agyo notebook add-source <id> "https://www.youtube.com/watch?v=..."

Run 'agyo notebook <subcommand> --help' for details on each subcommand.`)
}
