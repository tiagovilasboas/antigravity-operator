# Lista de Tarefas da Sessão

## Em Progresso ⏳
- [ ] Validar fluxo ponta a ponta com o usuário (abertura do NotebookLM e login Google)

## Pendentes 📋
- [ ] Divulgar no TabNews e LinkedIn

### Lifecycle hooks - plano: [docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md](../../docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md)
> 📦 *Histórico compactado: 7 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*
- [x] HK-4 Sessão por conversa: registrar `conversationId -> workspace` e avisar conflito
- [x] HK-5 Sugerir `session compact` via `PreInvocation` quando footprint passar da zona amarela
- [x] HK-6 Checkpoint automático em `PreToolUse` destrutivo (com bugs #18, #19, #21 resolvidos)
- [x] HK-7 Decidir dono dos hooks: coexistência limpa no `hooks.json` preservando third-party e custom hooks
- [x] Bug #18 rollback apaga sessão · #19 HEAD detached · #21 colisão de IDs de checkpoint (resolvidos nos PRs #28, #29, #30)
- [ ] Spike: existe modo headless (estilo `claude -p`) no CLI do Antigravity?

### UX da CLI - plano: [docs/plans/02-CLI-UX-CONTRATO.md](../../docs/plans/02-CLI-UX-CONTRATO.md)
> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*
- [x] UX-4 Ajuda curada por grupo no formato do `notebooklm` com exemplos (PR 2)
- [x] UX-7 `--json` em `version`, `browser tabs/status`, `notebooklm list/status`, `session list` (PR 2)
- [x] UX-3 Parser do `state.md` avisar formato não reconhecido (PR 3)
- [x] UX-9 Sugestão "você quis dizer" para comando desconhecido (PR 3)
- [x] UX-10 Confirmação/`--yes` em `rollback` sem id (PR 3)

### Release, sync e qualidade - plano: [docs/plans/03-RELEASE-SYNC-QUALIDADE.md](../../docs/plans/03-RELEASE-SYNC-QUALIDADE.md)
- [x] G5 🟡 Medir ou remover "~99% token savings" do README (nota metodológica FinOps documentada nos READMEs)
- [x] G4 🟡 Cobertura: `internal/notebook` 31% -> 84.5%; `cmd/agyo` 49% -> 73.6%
- [x] G7 🟢 `windows-latest` no CI ou marcar Windows como não testado (WSL2 documentado como alvo suportado e CI protegido)

### Fora do agyo (Downshift)
- [ ] DS-1 Reinstalar `downshift`: binário instalado imprime percentuais no `context benchmark` que contradizem o README
- [ ] DS-2 Validar build do `dsmon` (interrompido pelo restart)

## Concluídas ✅
> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-08.md](archive/completed-tasks-2026-10-08.md)*
> 📦 *Histórico compactado: 32 tarefas concluídas arquivadas em [completed-tasks-2026-10-01.md](archive/completed-tasks-2026-10-01.md)*
> 📦 *Histórico compactado: 5 tarefas concluídas arquivadas em [completed-tasks-2026-10-08.md](archive/completed-tasks-2026-10-08.md)*
- [x] PR #31 aprovado pelo CI do GitHub Actions e mesclado na branch `main`
- [x] Binário compilado e instalado localmente em `~/.local/bin/agyo` (v0.5.0)
- [x] README.md e README.pt-BR.md refatorados para layout moderno e escaneável (176 linhas, diagrama Brain & Hands, tabela FinOps e tabela de CLI com links para specs; CI 100% verde)

