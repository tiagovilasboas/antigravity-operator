package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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

func printGoogleMcpUsage() {
	fmt.Println(`Usage:
  agyo google mcp [action]

Actions:
  install     Configure google-calendar MCP server in Antigravity (mcp_config.json & default-servers.json)
  status      Check whether google-calendar MCP server is configured and active
  uninstall   Deregister google-calendar MCP server from Antigravity

Without action, 'agyo google mcp' starts the stdio MCP server for direct agent communication with zero browser tabs.`)
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
