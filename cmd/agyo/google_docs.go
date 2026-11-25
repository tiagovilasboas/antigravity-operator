package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/workspace"
)

func printGoogleDocsUsage() {
	fmt.Println(`Usage:
  agyo google docs <action> [arguments]

Actions:
  read <documentId-or-url> [--json]
      Read structured text from a Google Doc
  create "<title>" [--content "<text>"] [--json]
      Create a new Google Doc with optional initial text`)
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
