# Plano 06 - Arquitetura Agentic IA BDI, SRP, DRY, Clean Code e UX

> Status: concluido · Data: 2026-10-16 · Dono: Tiago Vilas Boas
> Tasks: `.agents/session/todo.md` (prefixos `BDI-`, `FIX-`, `REF-`)

## Contexto e Motivacao

Apos a entrega do Plano 05, um code review Senior Staff IA sob as lentes de **SRP, DRY, Clean Code, UX e Arquitetura Cognitiva BDI (Belief-Desire-Intention)** identificou oportunidades de evolucao para elevar o `agyo` de 8.4 para 9.5+:

1. **Modelo Cognitivo BDI explicito nos READMEs:** documentar visualmente como o `agyo` implementa o ciclo **Beliefs (Crencas)**, **Desires (Desejos)** e **Intentions (Intencoes)** governado pelo Outer Harness (Guias e Sensores de Fowler).
2. **UX de Flags Posicionais (`FIX-01`):** na stdlib `flag` do Go, passar um argumento posicional antes das flags (ex.: `agyo study ingest <url> --notebook <id>`) interrompe o parse das flags seguintes. Precisamos permutar argumentos posicionais e flags de forma transparente.
3. **Contrato Real dos Lifecycle Hooks (`FIX-02`):**
   - `PreInvocation` e `Stop` no `hooks.json` do Antigravity usam lista plana (`[]AntigravityHookCommand`), enquanto `PreToolUse` e `PostToolUse` usam lista agrupada por `matcher` (`[]AntigravityHookGroup`).
   - `PreToolUse` exige `decision: "deny"` ou `"allow"` (e nao `"block"`).
   - `PostToolUse` no Antigravity 2.0 nao envia `toolCall` no stdin; portanto o rastreamento de `DirtyCode` deve ocorrer tambem no `PreToolUse` (ou via leitura do transcript).
4. **Guardrail de Janela Unica no `google auth` (`FIX-03`):** priorizar `profile.OpenOrReuseTab` quando o Chrome isolado estiver ativo, evitando abrir janelas fantasmas.
5. **DRY (`REF-01`):** unificar o leitor de linhas JSONL ate 10 MB em `internal/watcher` (reutilizado por `exporter` e `analytics`) e centralizar o provedor de token OAuth com auto-refresh em `internal/workspace`.
6. **SRP (`REF-02`):** fatiar o God File `cmd/agyo/main.go` (1.774 linhas) em arquivos coesos por subcomando (`cmd_session.go`, `cmd_browser.go`, `cmd_notebook.go`, `cmd_checkpoint.go`, `cmd_hook.go`).

---

## Tasks Estruturadas

- [x] **BDI-01:** Criar diagrama de Arquitetura Agentic IA BDI (Belief-Desire-Intention + Outer Harness) em `README.md` e `README.pt-BR.md` mantendo paridade estrutural (`TestReadmeParity`).
- [x] **FIX-01:** Implementar permutacao inteligente de argumentos posicionais e flags (`permuteArgs`) em `cmd/agyo/study.go` e `cmd/agyo/google.go` + testes unitarios.
- [x] **FIX-02:** Corrigir contrato de `hooks.json` (flat vs grouped), enum `decision` (`deny`/`allow`) em `PreToolUse` e rastreamento de `DirtyCode` em `internal/hook/antigravity.go` + testes unitarios.
- [x] **FIX-03:** Alinhar `agyo google auth` ao Guardrail de Instancia Unica reutilizando aba do Chrome isolado quando ativo.
- [x] **REF-01:** Unificar leitor JSONL de 10 MB (`watcher.ForEachLine`) em `watcher`, `exporter` e `analytics`, e centralizar `workspace.GetValidAccessToken` em `internal/workspace/auth.go`.
- [x] **REF-02:** Fatiar `cmd/agyo/main.go` por dominio (`cmd_session.go`, `cmd_browser.go`, `cmd_notebook.go`, `cmd_checkpoint.go`, `cmd_hook.go`), mantendo `main.go` enxuto (< 300 linhas).
- [x] **QA-06:** Executar Quality Gate completo com `go test -count=1 -race ./...`, `go vet ./...`, validacao de paridade de docs e CI 100% verde no GitHub Actions.
