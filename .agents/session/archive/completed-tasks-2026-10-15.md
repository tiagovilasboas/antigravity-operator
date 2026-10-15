
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
