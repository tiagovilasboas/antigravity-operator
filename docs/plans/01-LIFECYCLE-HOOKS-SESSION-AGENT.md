# Plano 01 - Lifecycle hooks: agyo como agente de sessão de verdade

> Status: proposto · Data: 2026-10-15 · Dono: Tiago Vilas Boas
> Tasks: `.agents/session/todo.md` (prefixo `HK-`)

## Problema

Hoje o agyo opera **ao lado** do Antigravity, não **dentro** do loop do agente:

- `agyo sync` grava uma regra (`~/.gemini/antigravity/rules/session-agent.md`), uma skill e o MCP do NotebookLM. Tudo é **guia** (feedforward em texto). Nada obriga o modelo a ler ou atualizar `.agents/session/`.
- `session watch` / `dashboard` leem `transcript.jsonl` depois do fato: observam, não atuam.
- O único hook instalado pelo agyo é o `pre-commit` do git.

Evidências (2026-10-15):

- `grep -rn PreInvocation\|PostInvocation\|hooks.json --include='*.go'` no repo: zero resultados.
- `~/.gemini/config/hooks.json` contém apenas `downshift-model-router`, `secret-and-integrity-sensor` e `destructive-pre-tool` (scripts do harness-core). Nenhuma entrada do agyo.
- `internal/installer/installer.go` (`Sync`, L22-97): só rules, MCP e skill.

No Claude Code, o que faz o agente de sessão ser confiável é o harness (hooks `SessionStart`, `Stop`, `PostToolUse`), não a obediência do modelo.

## O que o Antigravity já oferece

Fonte: `~/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md`.

| Evento | Capacidade | Uso no agyo |
|---|---|---|
| `PreInvocation` | `injectSteps` (`ephemeralMessage`, `userMessage`, `toolCall`) | Injetar `state.md` + `todo.md` (equivalente a SessionStart) |
| `PreToolUse` | `allow` / `deny` / `ask` / `force_ask` + `overwrite` | Checkpoint automático antes de comando destrutivo |
| `PostToolUse` | recebe `stepIdx`, `error` | Marcar "editou código sem verificar"; atualizar footprint |
| `PostInvocation` | `force_continue` / `terminate` | Reservado (não usar no corte mínimo) |
| `Stop` | `decision: continue` + `reason` | Bloquear término com tarefa em progresso ou sem verificação |

Payload comum: `conversationId`, `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName` (camelCase).
Limitações: só `type: command`; síncrono (bloqueia o loop); timeout padrão 30 s.

## Desenho

Novo grupo de comandos, sem dependências externas:

```
agyo hook antigravity install   [--scope user|workspace] [--dry-run]
agyo hook antigravity uninstall
agyo hook antigravity status
agyo hook run <event>           # entrypoint chamado pelo hooks.json (stdin JSON -> stdout JSON)
```

- `install` faz merge de **uma** entrada nomeada `agyo-session-agent` no `hooks.json`, com backup datado. Não toca nas entradas do harness-core nem do downshift (o `hooks.json` já faz merge de hooks nomeados).
- `hook run` é fail-open: qualquer erro interno devolve a resposta neutra do evento (`{}`, `allow`, ou deixa parar) e loga em `~/.agyo/hook-errors.log`. O hook nunca pode travar o agente.
- Orçamento: < 200 ms por evento (o hook é síncrono).

### HK-1 `PreInvocation` - memória determinística

- Quando `invocationNum == 1` (ou ao mudar de `conversationId`): injeta um `ephemeralMessage` com objetivo, status, itens em progresso e próximos passos (resumo, não o arquivo inteiro).
- Teto de tamanho: 1.5 KB. Acima disso, injeta o resumo e sugere `agyo session compact`.

### HK-2 `Stop` - validação rigorosa como sensor

- Bloqueia (`decision: continue`) quando:
  1. houver item em `## Em Progresso` no `todo.md`; ou
  2. houve escrita de arquivo de código após o último comando de teste/build (estado mantido pelo HK-3).
- Anti-loop: no máximo 2 bloqueios por `conversationId` (contador em `~/.agyo/hooks/<conversationId>.json`). Ignora quando `terminationReason != model_stop`.
- `reason` diz exatamente o que falta ("todo.md tem 1 item em progresso: X").

### HK-3 `PostToolUse` - rastro de verificação

- Em `write_to_file` / `replace_file_content` em arquivo de código: marca `dirty=true`.
- Em `run_command` cujo comando casa com `go test|make test|npm test|pytest|go build|tsc` sem `error`: marca `dirty=false`.
- Estado por `conversationId`, nunca no repo.

### HK-4 Sessão por conversa

- Problema: `.agents/session/` é por workspace; duas conversas no mesmo repo disputam o mesmo `state.md`.
- Proposta mínima: os hooks registram `conversationId -> workspace` e o `state.md` ganha um campo `Conversa ativa`. Conflito gera aviso no `PreInvocation`, não bloqueio.
- Fora do escopo: um diretório de sessão por conversa (YAGNI até haver dor real).

### HK-5 Compactação automática

- O Antigravity não tem `PreCompact`. No `PreInvocation`, se o footprint de `state.md` + `todo.md` passar do limite atual do `doctor` (zona amarela), o hook injeta uma sugestão. Não compacta sozinho.

### HK-6 Checkpoint automático (depende dos bugs)

- `PreToolUse` em `run_command` destrutivo cria um checkpoint antes.
- **Bloqueado por** #18 (rollback apaga a sessão), #19 (HEAD detached) e #21 (colisão de IDs). Não ligar antes de corrigir.

### HK-7 Dono dos hooks

- Decidir: o agyo absorve `harness-pre-tool`/`harness-post-tool` ou só coexiste. Recomendação: coexistir no corte 1 e reavaliar depois.

## Fora de escopo

- Modo headless estilo `claude -p`: não verificado no CLI do Antigravity. Abrir spike antes de prometer.
- Hooks HTTP/prompt (não suportados pela plataforma).

## Critérios de aceite (corte mínimo = HK-1, HK-2, HK-3)

1. `agyo hook antigravity install --dry-run` mostra o diff do `hooks.json`; `install` real preserva as 3 entradas existentes (teste de golden file).
2. Conversa nova no Antigravity recebe o resumo do `state.md` sem o usuário pedir (evidência: `transcript.jsonl` com o `ephemeralMessage`).
3. Com 1 item em `## Em Progresso`, o agente é impedido de parar 1x com o `reason` correto; na 3a tentativa ele para (anti-loop).
4. Editar `.go` e tentar parar sem `go test` gera bloqueio; após `go test` verde, para normalmente.
5. Hook com `todo.md` corrompido ou ausente: resposta neutra, exit 0, erro logado (fail-open).
6. `go test -race ./...` verde; cobertura dos handlers `hook run` >= 80%.
7. `agyo doctor` ganha check "Lifecycle hooks: instalados / ausentes / desatualizados".

## Ordem sugerida

1. HK-1 + HK-3 + HK-2 (um PR, atrás de `install`, desligado por padrão).
2. `doctor` check + docs (`docs/spec/09-...`).
3. Bugs #18/#19/#21, depois HK-6.
4. HK-4, HK-5, HK-7.
