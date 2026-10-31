package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/computer"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func runComputer(info *platform.Info, args []string) {
	if len(args) == 0 {
		printComputerUsage()
		exitFunc(ExitUsageError)
		return
	}

	sub := args[0]
	if isHelp(sub) {
		printComputerUsage()
		return
	}

	engine := computer.NewEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch sub {
	case "inspect":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer inspect [--json]")
			fmt.Println("Inspects the active window and prints its semantic accessibility tree.")
			return
		}
		snap, err := engine.Inspect(ctx)
		if err != nil {
			runtimeError(err, "Certifique-se de que o Terminal/agyo possui permissão de Acessibilidade no macOS.")
			return
		}
		if hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(snap, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Print(snap.ToText())

	case "shot", "screenshot":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer shot [--window] [--format=jpeg|png] [--max-width=1280] [--quality=75] [output_file]")
			fmt.Println("Captures a compressed, token-optimized screenshot of the active window or screen.")
			return
		}
		opts := computer.ShotOptions{
			WindowOnly: true,
			Format:     "jpeg",
			MaxWidth:   1280,
			Quality:    75,
		}
		var outFile string
		for _, arg := range args[1:] {
			if arg == "--screen" || arg == "--full" {
				opts.WindowOnly = false
			} else if arg == "--window" {
				opts.WindowOnly = true
			} else if strings.HasPrefix(arg, "--format=") {
				opts.Format = strings.TrimPrefix(arg, "--format=")
			} else if strings.HasPrefix(arg, "--max-width=") {
				if w, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-width=")); err == nil {
					opts.MaxWidth = w
				}
			} else if strings.HasPrefix(arg, "--quality=") {
				if q, err := strconv.Atoi(strings.TrimPrefix(arg, "--quality=")); err == nil {
					opts.Quality = q
				}
			} else if !strings.HasPrefix(arg, "-") && outFile == "" {
				outFile = arg
			}
		}
		opts.OutPath = outFile
		data, savedPath, err := engine.Capture(ctx, opts)
		if err != nil {
			runtimeError(err, "Falha na captura de tela.")
			return
		}
		fmt.Printf("✅ Captura concluída (%d bytes): %s\n", len(data), savedPath)

	case "click":
		if len(args) < 2 || hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer click <ref|x> [y] [--double] [--right]")
			fmt.Println("Clicks on an element by reference (e.g. ref_1) or absolute (x, y) coordinates.")
			return
		}
		isDouble := false
		isRight := false
		var cleanArgs []string
		for _, a := range args[1:] {
			if a == "--double" {
				isDouble = true
			} else if a == "--right" {
				isRight = true
			} else if !strings.HasPrefix(a, "-") {
				cleanArgs = append(cleanArgs, a)
			}
		}

		action := computer.Action{Type: computer.ActionClick}
		if isDouble {
			action.Type = computer.ActionDoubleClick
		} else if isRight {
			action.Type = computer.ActionRightClick
		}

		if len(cleanArgs) == 1 {
			// Click por Ref
			action.Ref = cleanArgs[0]
		} else if len(cleanArgs) >= 2 {
			// Click por coordenadas X, Y
			x, errX := strconv.Atoi(cleanArgs[0])
			y, errY := strconv.Atoi(cleanArgs[1])
			if errX != nil || errY != nil {
				usageError("Coordenadas inválidas: x e y devem ser números inteiros")
				return
			}
			action.X = x
			action.Y = y
		} else {
			usageError("Especifique um ref (ex: ref_1) ou coordenadas (x y)")
			return
		}

		if err := engine.Execute(ctx, action); err != nil {
			runtimeError(err, "Falha ao executar clique.")
			return
		}
		if action.Ref != "" {
			fmt.Printf("✅ Clique executado no elemento %s\n", action.Ref)
		} else {
			fmt.Printf("✅ Clique executado em (%d, %d)\n", action.X, action.Y)
		}

	case "move":
		if len(args) < 3 || hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer move <x> <y>")
			return
		}
		x, errX := strconv.Atoi(args[1])
		y, errY := strconv.Atoi(args[2])
		if errX != nil || errY != nil {
			usageError("Coordenadas inválidas: x e y devem ser números inteiros")
			return
		}
		if err := engine.Execute(ctx, computer.Action{Type: computer.ActionMove, X: x, Y: y}); err != nil {
			runtimeError(err, "Falha ao mover cursor.")
			return
		}
		fmt.Printf("✅ Cursor movido para (%d, %d)\n", x, y)

	case "type":
		if len(args) < 2 || hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer type \"<text>\"")
			return
		}
		text := strings.Join(args[1:], " ")
		if err := engine.Execute(ctx, computer.Action{Type: computer.ActionTypeString, Text: text}); err != nil {
			runtimeError(err, "Falha ao digitar texto.")
			return
		}
		fmt.Printf("✅ Texto digitado com sucesso\n")

	case "key":
		if len(args) < 2 || hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer key \"<key_combo>\"")
			fmt.Println("Examples: agyo computer key return, agyo computer key cmd+space")
			return
		}
		combo := args[1]
		if err := engine.Execute(ctx, computer.Action{Type: computer.ActionShortcut, Key: combo}); err != nil {
			runtimeError(err, "Falha ao acionar atalho.")
			return
		}
		fmt.Printf("✅ Tecla/atalho %q acionado com sucesso\n", combo)

	case "batch":
		if len(args) < 2 || hasHelp(args[1:]) {
			fmt.Println("Usage: agyo computer batch '<json_array>'")
			return
		}
		var actions []computer.Action
		if err := json.Unmarshal([]byte(args[1]), &actions); err != nil {
			usageError("JSON inválido para lote de ações: %v", err)
			return
		}
		if err := engine.ExecuteBatch(ctx, actions); err != nil {
			runtimeError(err, "Falha na execução da macro.")
			return
		}
		fmt.Printf("✅ Lote de %d ações executado com sucesso\n", len(actions))

	case "mcp":
		if err := computer.ServeMCP(engine, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Erro no servidor MCP: %v\n", err)
			exitFunc(ExitRuntimeError)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown computer command: %q\n", sub)
		printComputerUsage()
		exitFunc(ExitUsageError)
	}
}

func printComputerUsage() {
	fmt.Println(`agyo computer - High-Performance, Token-Efficient Computer Use Engine

Usage:
  agyo computer <subcommand> [options]

Subcommands:
  inspect [--json]              Inspect active window and extract semantic Accessibility Tree
  shot [options] [out_file]     Capture compressed, token-optimized screenshot (JPEG 1280px by default)
                                Options: --window (default), --screen, --format=jpeg|png, --quality=75
  click <ref|x> [y] [flags]     Click on element by ref (e.g. ref_1) or coordinates (x y)
                                Flags: --double, --right
  move <x> <y>                  Move mouse cursor to coordinates
  type "<text>"                 Type text string into focused element
  key "<key_combo>"             Press key or shortcut (e.g. return, esc, cmd+space, cmd+c)
  batch '<json_array>'          Execute an atomic macro sequence of actions in one call
  mcp                           Start Model Context Protocol (MCP) stdio server for agents`)
}
