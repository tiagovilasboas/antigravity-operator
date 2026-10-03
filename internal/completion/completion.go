package completion

import (
	"fmt"
	"io"
	"strings"
)

// Generate escreve o script de autocompletar para a shell especificada.
func Generate(shell string, w io.Writer) error {
	switch strings.ToLower(shell) {
	case "bash":
		_, err := fmt.Fprint(w, bashScript)
		return err
	case "zsh":
		_, err := fmt.Fprint(w, zshScript)
		return err
	case "fish":
		_, err := fmt.Fprint(w, fishScript)
		return err
	default:
		return fmt.Errorf("shell não suportada: %q (use 'bash', 'zsh' ou 'fish')", shell)
	}
}

const bashScript = `# bash completion for agyo
_agyo_completion() {
    local cur prev words cword
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion || return
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local commands="init session checkpoint rollback dashboard doctor browser sync hook completion about version help"
    local session_subcommands="status compact archive list restore watch export"
    local browser_subcommands="start status stop tabs open close eval shot"
    local hook_subcommands="install uninstall"
    local completion_subcommands="bash zsh fish"

    if [ $cword -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
        return 0
    fi

    case "${words[1]}" in
        session)
            if [ $cword -eq 2 ]; then
                COMPREPLY=( $(compgen -W "${session_subcommands}" -- ${cur}) )
            fi
            ;;
        browser)
            if [ $cword -eq 2 ]; then
                COMPREPLY=( $(compgen -W "${browser_subcommands}" -- ${cur}) )
            fi
            ;;
        hook)
            if [ $cword -eq 2 ]; then
                COMPREPLY=( $(compgen -W "${hook_subcommands}" -- ${cur}) )
            fi
            ;;
        completion)
            if [ $cword -eq 2 ]; then
                COMPREPLY=( $(compgen -W "${completion_subcommands}" -- ${cur}) )
            fi
            ;;
        sync)
            COMPREPLY=( $(compgen -W "--update-mcp" -- ${cur}) )
            ;;
    esac
}
complete -F _agyo_completion agyo
`

const zshScript = `#compdef agyo

_agyo() {
    local -a commands session_cmds browser_cmds hook_cmds completion_cmds

    commands=(
        'init:Scaffold operational memory (.agents/session/) in the target project'
        'session:Manage operational memory, progress, streaming, and export'
        'checkpoint:Create an atomic filesystem snapshot before risky refactoring'
        'rollback:Safely undo agent edits and restore exact working tree from a checkpoint'
        'dashboard:Launch local web dashboard for live monitoring and DevTools'
        'doctor:Audit host readiness (OS, Chrome, DevTools 9222, Git, Node/NPX)'
        'browser:Manage isolated Chrome process lifecycle and CDP'
        'sync:Synchronize canonical rules and MCP manifests to Google Antigravity'
        'hook:Install or uninstall git pre-commit session continuity hook'
        'completion:Generate shell autocompletion script (bash, zsh, fish)'
        'about:Display manifesto and tribute to the community & Google AI Pro'
        'version:Print version and system architecture'
        'help:Show help information'
    )

    session_cmds=(
        'status:Display active session objective, status, and task metrics'
        'compact:Archive completed tasks and rollup active todo.md to avoid context bloat'
        'archive:Archive completed session and reset templates'
        'list:List all archived historical sessions'
        'restore:Restore a past archived session into active memory with automatic backup'
        'watch:Stream agent reasoning, subagents, and desktop notifications'
        'export:Export consolidated session report in markdown or HTML'
    )

    browser_cmds=(
        'start:Launch isolated Chrome instance with remote debugging flags'
        'status:Inspect DevTools port (9222) readiness and Chrome PID'
        'stop:Gracefully terminate isolated Chrome process (SIGTERM)'
        'tabs:List all open tabs and target IDs in isolated Chrome'
        'open:Open a new tab at given URL in isolated Chrome'
        'close:Close target tab by ID'
        'eval:Evaluate JavaScript expression in active tab'
        'shot:Capture PNG screenshot of active tab'
    )

    hook_cmds=(
        'install:Install git pre-commit hook to safeguard session continuity'
        'uninstall:Remove agyo git pre-commit hook'
    )

    completion_cmds=(
        'bash:Generate bash autocompletion script'
        'zsh:Generate zsh autocompletion script'
        'fish:Generate fish autocompletion script'
    )

    _arguments -C \
        '1: :->command' \
        '2: :->subcommand' \
        '*:: :->args'

    case $state in
        command)
            _describe -t commands 'agyo command' commands
            ;;
        subcommand)
            case $words[2] in
                session)
                    _describe -t session_cmds 'session subcommand' session_cmds
                    ;;
                browser)
                    _describe -t browser_cmds 'browser subcommand' browser_cmds
                    ;;
                hook)
                    _describe -t hook_cmds 'hook subcommand' hook_cmds
                    ;;
                completion)
                    _describe -t completion_cmds 'shell type' completion_cmds
                    ;;
                sync)
                    _values 'sync flag' '--update-mcp[Back up and rewrite the MCP manifest from the built-in template]'
                    ;;
            esac
            ;;
    esac
}

_agyo "$@"
`

const fishScript = `# fish completion for agyo
function __agyo_needs_command
    set -l cmd (commandline -opc)
    if [ (count $cmd) -eq 1 ]
        return 0
    end
    return 1
end

complete -c agyo -n '__agyo_needs_command' -a 'init' -d 'Scaffold operational memory'
complete -c agyo -n '__agyo_needs_command' -a 'session' -d 'Manage operational memory and streaming'
complete -c agyo -n '__agyo_needs_command' -a 'checkpoint' -d 'Create an atomic filesystem snapshot'
complete -c agyo -n '__agyo_needs_command' -a 'rollback' -d 'Safely undo agent edits and restore from checkpoint'
complete -c agyo -n '__agyo_needs_command' -a 'dashboard' -d 'Launch local web dashboard'
complete -c agyo -n '__agyo_needs_command' -a 'doctor' -d 'Audit host readiness'
complete -c agyo -n '__agyo_needs_command' -a 'browser' -d 'Manage isolated Chrome and CDP'
complete -c agyo -n '__agyo_needs_command' -a 'sync' -d 'Synchronize rules and MCP manifests'
complete -c agyo -n '__agyo_needs_command' -a 'hook' -d 'Manage git pre-commit hook'
complete -c agyo -n '__agyo_needs_command' -a 'completion' -d 'Generate shell completion script'
complete -c agyo -n '__agyo_needs_command' -a 'about' -d 'Display project manifesto'
complete -c agyo -n '__agyo_needs_command' -a 'version' -d 'Print version'

# Subcommands
complete -c agyo -n '__fish_seen_subcommand_from session' -a 'status compact archive list restore watch export'
complete -c agyo -n '__fish_seen_subcommand_from browser' -a 'start status stop tabs open close eval shot'
complete -c agyo -n '__fish_seen_subcommand_from hook' -a 'install uninstall'
complete -c agyo -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c agyo -n '__fish_seen_subcommand_from sync' -l update-mcp -d 'Back up and rewrite the MCP manifest'
`
