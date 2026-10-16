
### 📦 Tarefas Compactadas em 2026-10-16 09:57:04 (10 itens)
- [x] Spike: existe modo headless (estilo `claude -p`) no CLI do Antigravity? (verificado via `language_server agentapi` e `gemini -p`; documentado na Spec 09)
- [x] BDI-01: Diagrama Agentic IA BDI (Belief-Desire-Intention) em `README.md` e `README.pt-BR.md`
- [x] FIX-01: Permutacao de flags posicionais (`permuteArgs`) em `study.go` e `google.go` + testes
- [x] FIX-02: Correcao do contrato de `hooks.json` (flat vs grouped), `decision: deny/allow` e `DirtyCode` em `internal/hook/antigravity.go`
- [x] FIX-03: Alinhar `agyo google auth` ao Guardrail de Instancia Unica (`profile.OpenOrReuseTab`)
- [x] REF-01: DRY do leitor JSONL 10 MB (`watcher.ForEachLine`) e `workspace.GetValidAccessToken`
- [x] REF-02: SRP split de `cmd/agyo/main.go` em `cmd_session.go`, `cmd_browser.go`, `cmd_notebook.go`, `cmd_checkpoint.go`, `cmd_hook.go`
- [x] QA-06: Quality Gate completo (`go test -count=1 -race ./...` + `go vet ./...`)
- [x] DS-1 Reinstalar `downshift`: resolvido pela instalação da v0.1.0-beta.14 em `~/go/bin` (2026-10-15). Os percentuais vinham de builds sem tag de 2026-10-08; código, README e teste concordam (tabela de bytes medidos). Binários antigos sombreados em `~/.local/bin` e `/opt/homebrew/bin`.
- [x] DS-2 Validar build do `dsmon`: build, vet e testes passam a partir de um clone limpo (2026-10-16). O hook do Kiro apontava para um `dsmon-hook` de 2026-09-30 que gravava texto do prompt; recompilado da main atual, junto com `dsmon`, `dsmon-server` e o `downshift` usado pelo LaunchAgent.
