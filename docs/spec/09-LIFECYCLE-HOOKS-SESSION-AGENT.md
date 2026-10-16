# Spec 09: Lifecycle Hooks & Autonomous Session Agent Engine

## Visão Geral

O módulo de Lifecycle Hooks conecta o `agyo` diretamente ao ciclo de execução interno do Google Antigravity, transformando regras passivas de texto em garantias determinísticas de engenharia (Outer Harness - Martin Fowler).

Em vez de depender apenas da adesão inferencial do modelo, o harness do `agyo` atua como um sensor e guia ativo:
1. **PreInvocation (HK-1):** Injeta o resumo da memória de sessão (`state.md` + `todo.md`) na primeira chamada de contexto da conversa via `ephemeralMessage`.
2. **PostToolUse (HK-3):** Rastreia modificações de código fonte executável (`.go`, `.ts`, `.py`, etc.) e limpa o flag de pendência de verificação quando comandos de teste (`go test`, `npm test`, `pytest`) ou compilação passam com sucesso.
3. **Stop (HK-2):** Impede a parada prematura do agente (`decision: continue`) se restarem tarefas marcadas em `## Em Progresso` no `todo.md` ou se houver código editado sem verificação, com proteção anti-loop de no máximo 2 bloqueios.

---

## Comandos da CLI

### 1. Registro e Gerenciamento no Antigravity (`hooks.json`)

```bash
# Registra o hook com fusão não destrutiva e backup automático
agyo hook antigravity install [--scope user|workspace]

# Visualiza o diff de modificação sem gravar no disco
agyo hook antigravity install --dry-run

# Verifica se o hook agyo-session-agent está ativo
agyo hook antigravity status [--scope user|workspace]

# Remove o hook preservando hooks de terceiros no hooks.json
agyo hook antigravity uninstall [--scope user|workspace]
```

### 2. Executor de Eventos (Invocado pelo Antigravity)

```bash
agyo hook run PreInvocation
agyo hook run PostToolUse
agyo hook run Stop
```

O comando `agyo hook run` consome JSON via `stdin` e responde JSON via `stdout`. Qualquer erro interno é estritamente fail-open (retorna resposta neutra com exit code 0 e registra o evento em `~/.agyo/hook-errors.log`), garantindo que o agente nunca seja bloqueado por falha de infraestrutura.

---

## Contrato de Eventos

### PreInvocation (HK-1)

- **Gatilho:** Invocação 1 (`invocationNum == 1`).
- **Comportamento:** Localiza o workspace ativo e lê `.agents/session/`. Injeta mensagem efêmera com Objetivo, Status/Fase e Tarefas em Progresso.
- **Teto de Contexto:** Orçamento de 1.5 KB. Acima deste limite, trunca e alerta a necessidade de `agyo session compact`.

### PostToolUse (HK-3)

- **Gatilho:** Execução de ferramentas pelo modelo.
- **Detecção de Edição:** Ferramentas `write_to_file` ou `replace_file_content` modificando arquivos de código (`.go`, `.ts`, `.tsx`, `.js`, `.py`, `.rs`, `.php`, etc.) marcam `dirtyCode = true`.
- **Detecção de Verificação:** Ferramentas `run_command` executando `go test`, `make test`, `npm test`, `pytest`, `cargo test`, `go build`, etc. com código de retorno 0 marcam `dirtyCode = false`.
- **Persistência:** Estado mantido isoladamente por conversa em `~/.agyo/hooks/<conversationId>.json`.

### Stop (HK-2)

- **Gatilho:** Modelo decide encerrar a execução (`terminationReason == "model_stop"`).
- **Validação de Tarefas:** Se houver itens pendentes sob a seção `## Em Progresso ⏳` no `todo.md`, bloqueia com instrução clara para concluir ou desmarcar.
- **Validação de Testes:** Se `dirtyCode == true`, bloqueia instruindo a rodar os testes de verificação antes de concluir.
- **Guardião Anti-loop:** No máximo 2 bloqueios consecutivos por conversa. Na terceira tentativa, o encerramento é liberado para prevenir loops infinitos.

---

## Auditoria via `agyo doctor`

O comando `agyo doctor` inspeciona a prontidão do lifecycle hook no sistema do usuário:
- `✅ Lifecycle Hooks : agyo-session-agent ativo em ~/.gemini/config/hooks.json`
- `ℹ️ Lifecycle Hooks : Não instalados em hooks.json (execute: agyo hook antigravity install)`

---

## Spike HK-8: Execução Headless e CLI Nativa do Antigravity

Em outubro de 2026, foi conduzido um spike aprofundado para investigar a viabilidade de execução não-interativa / headless no Google Antigravity (estilo `claude -p`).

### Descobertas de Engenharia

1. **Codinome agy:**
   - A sigla `agy` é a nomenclatura de engenharia interna do Google para o Antigravity (ex.: wrappers `agy-node`, configuracoes `agyAllowedModels`, skills `agy-customizations`).
2. **Motor Core (`language_server agentapi`):**
   - O binario `/Applications/Antigravity.app/Contents/Resources/bin/language_server` implementa nativamente o subcomando `agentapi`:
     - `agentapi new-conversation [--model=<flash_lite|flash|pro>] [--title=<title>] <prompt>`: cria uma sessao programaticamente com workspace resolvido e despacha a execucao em background.
     - `agentapi send-message <recipient_id> <content>`: envia mensagens e diretrizes diretamente a conversas ou subagentes ativos.
     - `agentapi get-conversation-metadata <conversation_id>`: extrai em tempo real o estado, arvore de subagentes, permissoes e transcricoes da sessao em formato JSON.
3. **Execucao Headless com Gemini CLI (`gemini -p`):**
   - O binario `@google/gemini-cli` instalado em `/opt/homebrew/bin/gemini` oferece suporte nativo ao estilo `claude -p`:
     ```bash
     cat prompt.md | gemini -p "" -y -o json
     ```
   - Auto-aprova ferramentas com `-y` / `--approval-mode yolo` e emite JSON estruturado com `-o json`.

### Interoperabilidade no Roadmap do agyo

- **Wrapper agy canonico:** O comando `agyo sync` pode provisionar um symlink ou executavel wrapper em `~/.local/bin/agy` delegando para `language_server agentapi`.
- **Subcomando agyo run (Outer Harness):** Executa pipelines de CI ou automacoes locais criando checkpoints de seguranca antes de disparar o agente de forma autonoma.

