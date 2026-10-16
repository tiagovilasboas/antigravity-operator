
### 📦 Tarefas Compactadas em 2026-10-15 02:57:58 (8 itens)
- [x] UX-1 `notebooklm open --help` abre o Chrome: interceptar `--help` antes de qualquer ação (PR 1)
- [x] UX-2 `--help` nos grupos `session`/`browser`/`hook`/`checkpoint` (PR 1)
- [x] UX-6 Contrato de exit code 0/1/2 + teste table-driven com Chrome desligado (PR 1)
- [x] UX-8 Helper único de erro com "→ próximo passo" (PR 1)
- [x] UX-5 Padronizar idioma das mensagens (decidir PT-BR ou EN) (PR 1)
- [x] G3 Mover `~/.gemini/config/skills/agyo.bak` para `~/.gemini/config/skills-backup/` + check "Agent Skills" no `doctor` detectando pastas .bak
- [x] G2 `sync` não escrever através de symlink (harness-core) + backup antes de sobrescrever rules/skill
- [x] Hardening de segurança AppSec aplicado (`--remote-debugging-address=127.0.0.1`, `0700` no perfil, validação de notebookID e fix DOM no `PushSource`)

### 📦 Tarefas Compactadas em 2026-10-15 03:09:37 (7 itens)
- [x] HK-1 `PreInvocation`: injetar resumo de `state.md` + `todo.md` na 1a invocação (teto 1.5 KB)
- [x] HK-2 `Stop`: bloquear término com item em progresso ou código sem verificação (anti-loop: máx. 2)
- [x] HK-3 `PostToolUse`: rastrear `dirty` (escrita de código) vs teste/build verde, por `conversationId`
- [x] HK-0 `agyo hook antigravity install|uninstall|status` + `agyo hook run <event>` (merge com backup, fail-open, < 200 ms)
- [x] HK-doctor: check "Lifecycle hooks" no `agyo doctor` + `docs/spec/09-...`
- [x] G1 🔴 Cortar `v0.5.0` (NotebookLM fora de qualquer release), atualizar Formula/tap, `make install` com `-ldflags`
- [x] G6 🟡 Fixar versões `npx` em `templates/mcps/default-servers.json`

### 📦 Tarefas Compactadas em 2026-10-15 22:08:11 (29 itens)
- [x] HK-4 Sessão por conversa: registrar `conversationId -> workspace` e avisar conflito
- [x] HK-5 Sugerir `session compact` via `PreInvocation` quando footprint passar da zona amarela
- [x] HK-6 Checkpoint automático em `PreToolUse` destrutivo (com bugs #18, #19, #21 resolvidos)
- [x] HK-7 Decidir dono dos hooks: coexistência limpa no `hooks.json` preservando third-party e custom hooks
- [x] Bug #18 rollback apaga sessão · #19 HEAD detached · #21 colisão de IDs de checkpoint (resolvidos nos PRs #28, #29, #30)
- [x] UX-4 Ajuda curada por grupo no formato do `notebooklm` com exemplos (PR 2)
- [x] UX-7 `--json` em `version`, `browser tabs/status`, `notebooklm list/status`, `session list` (PR 2)
- [x] UX-3 Parser do `state.md` avisar formato não reconhecido (PR 3)
- [x] UX-9 Sugestão "você quis dizer" para comando desconhecido (PR 3)
- [x] UX-10 Confirmação/`--yes` em `rollback` sem id (PR 3)
- [x] G5 🟡 Medir ou remover "~99% token savings" do README (nota metodológica FinOps documentada nos READMEs)
- [x] G4 🟡 Cobertura: `internal/notebook` 31% -> 84.5%; `cmd/agyo` 49% -> 73.6%
- [x] G7 🟢 `windows-latest` no CI ou marcar Windows como não testado (WSL2 documentado como alvo suportado e CI protegido)
- [x] WS-1 Auth & Storage: TokenInfo, Save/Load (0600), refresh automático, isolamento de contexto e loopback server seguro (SEC-WS-01, SEC-WS-02, SEC-WS-03)
- [x] WS-2 Calendar Client: Métodos ListEvents e CreateEvent para Google Calendar API v3 com timeout de 30s
- [x] WS-3 Docs Client: Métodos GetDocument e CreateDocument para Google Docs API v1 com tratamento de batchUpdate
- [x] WS-4 CLI Subcommands: Integrar `agyo google` (auth, status, calendar, docs) no dispatcher da CLI
- [x] WS-5 MCP Tools: Expor ferramentas do Workspace no servidor MCP stdio
- [x] WS-6 Testes & Qualidade: Cobertura 86.9% com httptest.Server e -race 100% verde
- [x] WS-7 Docs & Sync: Atualizar documentações e registrar Spec 10
- [x] GAP-01: Exporter com formato nativo de tool_calls (tc.Name) e leitor ate 10 MB sem travar
- [x] GAP-02: CDP WebSocket resiliente com deadline 15s e loop de matching id
- [x] GAP-03: NotebookLM PushSource com injecao real de title e content no DOM
- [x] GAP-04: NotebookLM AskNotebook com medicao de initialCount e estabilizacao de streaming
- [x] GAP-05: Embutir skills academicas no FS e instalar no agyo sync + check do Workspace no doctor
- [x] PERF-01: Cache com TTL no doctor do dashboard e janela deslizante no watcher
- [x] PERF-02: Fast-path de redacao de segredos no analytics
- [x] STU-01: Comando CLI `agyo study` (ingest, exam-prep, quiz) integrando NotebookLM + Calendar + Docs
- [x] STU-02: Testes unitarios com -race 100% verdes across all packages
