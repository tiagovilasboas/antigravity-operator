package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/checkpoint"
)

func checkpointUsageString() string {
	return `Usage: agyo checkpoint [name] [options] [dir]

Create an atomic filesystem snapshot before risky refactoring, or list saved checkpoints.

Options:
  -desc string  Optional human-readable description for the checkpoint
  -list         List all saved checkpoints in active session (.agents/session/checkpoints.json)
  -json         Output checkpoint list as machine-readable JSON (used with -list)

Examples:
  agyo checkpoint
  agyo checkpoint pre-refactor -desc "Before refactoring auth module"
  agyo checkpoint -list
  agyo checkpoint -list -json`
}

func printCheckpointUsage() {
	fmt.Println(checkpointUsageString())
}

func runCheckpoint(args []string) {
	if hasHelp(args) {
		printCheckpointUsage()
		return
	}
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
			runtimeError(err, "")
			return
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
		runtimeError(err, "")
		return
	}

	fmt.Printf("🛡️  Checkpoint criado com sucesso!\n")
	fmt.Printf("   ID        : %s\n", chk.ID)
	fmt.Printf("   Nome      : %s\n", chk.Name)
	fmt.Printf("   Branch    : %s (%s)\n", chk.Branch, chk.CommitSHA)
	if len(chk.DirtyFiles) > 0 {
		if chk.StashSHA != "" {
			fmt.Printf("   Modificados: alterações rastreadas preservadas no stash commit (%s)\n", chk.StashSHA)
		}
		if len(chk.Untracked) > 0 {
			fmt.Printf("   Não rastreados: %d arquivos mantidos no rollback (conteúdo não versionado no checkpoint)\n", len(chk.Untracked))
		}
	} else {
		fmt.Printf("   Modificados: Working tree limpa (clean tree)\n")
	}
	fmt.Println("💡 Em caso de falha ou refatoração indesejada: agyo rollback")
}

func rollbackUsageString() string {
	return `Usage: agyo rollback [checkpoint-id|latest] [options] [dir]

Safely undo agent edits and restore exact working tree from a checkpoint without detaching HEAD.

Options:
  --yes, -y    Skip interactive confirmation when rolling back to latest checkpoint

Examples:
  agyo rollback --yes
  agyo rollback latest --yes
  agyo rollback chk-20261015-025037`
}

func printRollbackUsage() {
	fmt.Println(rollbackUsageString())
}

func runRollback(args []string) {
	if hasHelp(args) {
		printRollbackUsage()
		return
	}
	targetID := ""
	targetDir := "."
	hasYes := false

	var cleanArgs []string
	for _, a := range args {
		if a == "--yes" || a == "-yes" || a == "-y" {
			hasYes = true
		} else {
			cleanArgs = append(cleanArgs, a)
		}
	}

	if len(cleanArgs) == 1 {
		// Se for um diretório existente, usa como targetDir, senão é targetID
		if fi, err := os.Stat(cleanArgs[0]); err == nil && fi.IsDir() {
			targetDir = cleanArgs[0]
		} else {
			targetID = cleanArgs[0]
		}
	} else if len(cleanArgs) > 1 {
		targetID = cleanArgs[0]
		targetDir = cleanArgs[1]
	}

	// UX-10: rollback sem ID explícito reverte para latest; exige confirmação em TTY ou --yes em non-TTY
	if (targetID == "" || targetID == "latest") && !hasYes {
		if !isTerminalFunc() {
			usageError("rollback to latest without checkpoint ID in non-interactive mode requires --yes flag (e.g. 'agyo rollback --yes')")
			return
		}

		fmt.Print("⚠️  Reverting to the latest checkpoint will overwrite local uncommitted changes. Proceed? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Rollback cancelled.")
			return
		}
	}

	res, err := checkpoint.Rollback(targetDir, targetID)
	if err != nil {
		runtimeError(err, "Run 'agyo checkpoint -list' to inspect available checkpoints")
		return
	}

	fmt.Printf("⏪ Workspace restaurado com sucesso para o checkpoint:\n")
	fmt.Printf("   ID        : %s\n", res.ID)
	fmt.Printf("   Nome      : %s\n", res.Name)
	fmt.Printf("   Timestamp : %s\n", res.Timestamp)
	fmt.Printf("   Branch    : %s (%s)\n", res.Branch, res.CommitSHA)
	if len(res.DirtyFiles) > 0 {
		fmt.Printf("   Arquivos  : %d modificações restauradas na working tree\n", len(res.DirtyFiles))
	}
	if w := res.Warning(); w != "" {
		fmt.Print(w)
	}
}
