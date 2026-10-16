# Lista de Tarefas da Sessão

## Em Progresso ⏳
- *(Nenhuma tarefa em execução)*

## Pendentes 📋
- [ ] Validar fluxo ponta a ponta com o usuário (abertura do NotebookLM e login Google)
- [ ] Divulgar no TabNews e LinkedIn

### Lifecycle hooks - plano: [docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md](../../docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md)
> 📦 *Histórico compactado: 7 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*
> 📦 *Histórico compactado: 29 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*
- [x] Spike: existe modo headless (estilo `claude -p`) no CLI do Antigravity? (verificado via `language_server agentapi` e `gemini -p`; documentado na Spec 09)

### UX da CLI - plano: [docs/plans/02-CLI-UX-CONTRATO.md](../../docs/plans/02-CLI-UX-CONTRATO.md)
> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*

### Release, sync e qualidade - plano: [docs/plans/03-RELEASE-SYNC-QUALIDADE.md](../../docs/plans/03-RELEASE-SYNC-QUALIDADE.md)

### Google Workspace Bridge - plano: [docs/plans/04-GOOGLE-WORKSPACE-BRIDGE.md](../../docs/plans/04-GOOGLE-WORKSPACE-BRIDGE.md)

### Gaps, Performance & Student Edition - plano: [docs/plans/05-GAPS-PERFORMANCE-STUDENT-EDITION.md](../../docs/plans/05-GAPS-PERFORMANCE-STUDENT-EDITION.md)

### Arquitetura Agentic IA BDI, SRP, DRY, Clean Code & UX - plano: [docs/plans/06-BDI-ARCHITECTURE-SRP-DRY-CLEAN-CODE.md](../../docs/plans/06-BDI-ARCHITECTURE-SRP-DRY-CLEAN-CODE.md)
- [x] BDI-01: Diagrama Agentic IA BDI (Belief-Desire-Intention) em `README.md` e `README.pt-BR.md`
- [x] FIX-01: Permutacao de flags posicionais (`permuteArgs`) em `study.go` e `google.go` + testes
- [x] FIX-02: Correcao do contrato de `hooks.json` (flat vs grouped), `decision: deny/allow` e `DirtyCode` em `internal/hook/antigravity.go`
- [x] FIX-03: Alinhar `agyo google auth` ao Guardrail de Instancia Unica (`profile.OpenOrReuseTab`)
- [x] REF-01: DRY do leitor JSONL 10 MB (`watcher.ForEachLine`) e `workspace.GetValidAccessToken`
- [x] REF-02: SRP split de `cmd/agyo/main.go` em `cmd_session.go`, `cmd_browser.go`, `cmd_notebook.go`, `cmd_checkpoint.go`, `cmd_hook.go`
- [x] QA-06: Quality Gate completo (`go test -count=1 -race ./...` + `go vet ./...`)

### Fora do agyo (Downshift)
- [x] DS-1 Reinstalar `downshift`: resolvido pela instalação da v0.1.0-beta.14 em `~/go/bin` (2026-10-15). Os percentuais vinham de builds sem tag de 2026-10-08; código, README e teste concordam (tabela de bytes medidos). Binários antigos sombreados em `~/.local/bin` e `/opt/homebrew/bin`.
- [x] DS-2 Validar build do `dsmon`: build, vet e testes passam a partir de um clone limpo (2026-10-16). O hook do Kiro apontava para um `dsmon-hook` de 2026-09-30 que gravava texto do prompt; recompilado da main atual, junto com `dsmon`, `dsmon-server` e o `downshift` usado pelo LaunchAgent.

## Concluídas ✅
> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-08.md](archive/completed-tasks-2026-10-08.md)*
> 📦 *Histórico compactado: 32 tarefas concluídas arquivadas em [completed-tasks-2026-10-01.md](archive/completed-tasks-2026-10-01.md)*
> 📦 *Histórico compactado: 5 tarefas concluídas arquivadas em [completed-tasks-2026-10-08.md](archive/completed-tasks-2026-10-08.md)*
- [x] PR #31 aprovado pelo CI do GitHub Actions e mesclado na branch `main`
- [x] Binário compilado e instalado localmente em `~/.local/bin/agyo` (v0.5.0)
- [x] README.md e README.pt-BR.md refatorados para layout moderno e escaneável (diagrama Brain & Hands, diagrama Agentic IA BDI, tabela FinOps e tabela de CLI com links para specs; CI 100% verde)

