# Lista de Tarefas da Sessão

## Em Progresso ⏳
- [ ] Validar fluxo ponta a ponta com o usuário (abertura do NotebookLM e login Google)

## Pendentes 📋
- [ ] Divulgar no TabNews e LinkedIn

### Lifecycle hooks - plano: [docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md](../../docs/plans/01-LIFECYCLE-HOOKS-SESSION-AGENT.md)
- [ ] HK-1 `PreInvocation`: injetar resumo de `state.md` + `todo.md` na 1a invocação (teto 1.5 KB)
- [ ] HK-2 `Stop`: bloquear término com item em progresso ou código sem verificação (anti-loop: máx. 2)
- [ ] HK-3 `PostToolUse`: rastrear `dirty` (escrita de código) vs teste/build verde, por `conversationId`
- [ ] HK-0 `agyo hook antigravity install|uninstall|status` + `agyo hook run <event>` (merge com backup, fail-open, < 200 ms)
- [ ] HK-doctor: check "Lifecycle hooks" no `agyo doctor` + `docs/spec/09-...`
- [ ] HK-4 Sessão por conversa: registrar `conversationId -> workspace` e avisar conflito
- [ ] HK-5 Sugerir `session compact` via `PreInvocation` quando footprint passar da zona amarela
- [ ] HK-6 Checkpoint automático em `PreToolUse` destrutivo (bloqueado por #18, #19, #21)
- [ ] HK-7 Decidir dono dos hooks (agyo vs `harness-pre-tool`/`harness-post-tool`)
- [ ] Bug #18 rollback apaga sessão · #19 HEAD detached · #21 colisão de IDs de checkpoint
- [ ] Spike: existe modo headless (estilo `claude -p`) no CLI do Antigravity?

### UX da CLI - plano: [docs/plans/02-CLI-UX-CONTRATO.md](../../docs/plans/02-CLI-UX-CONTRATO.md)
> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-15.md](archive/completed-tasks-2026-10-15.md)*
- [ ] UX-4 Ajuda curada por grupo no formato do `notebooklm` (PR 2)
- [ ] UX-7 `--json` em `version`, `browser tabs/status`, `notebooklm list/status`, `session list` (PR 2)
- [ ] UX-3 Parser do `state.md` avisar formato não reconhecido (PR 3)
- [ ] UX-9 Sugestão "você quis dizer" para comando desconhecido (PR 3)
- [ ] UX-10 Confirmação/`--yes` em `rollback` sem id (PR 3)

### Release, sync e qualidade - plano: [docs/plans/03-RELEASE-SYNC-QUALIDADE.md](../../docs/plans/03-RELEASE-SYNC-QUALIDADE.md)
- [x] G1 🔴 Cortar `v0.5.0` (NotebookLM fora de qualquer release), atualizar Formula/tap, `make install` com `-ldflags`
- [x] G6 🟡 Fixar versões `npx` em `templates/mcps/default-servers.json`
- [ ] G5 🟡 Medir ou remover "~99% token savings" do README
- [ ] G4 🟡 Cobertura: `internal/notebook` 31% -> 70% (fixtures HTML + smoke `AGYO_E2E=1`); `cmd/agyo` 49% -> 70%
- [ ] G7 🟢 `windows-latest` no CI ou marcar Windows como não testado

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

