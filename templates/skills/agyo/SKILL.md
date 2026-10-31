---
name: agyo
description: Operates the local Antigravity Operator CLI (agyo) - session memory lifecycle, host environment diagnostics, isolated Chrome supervision, native CDP browser inspection, and git pre-commit session continuity hooks. Activate when the user mentions "agyo", "status da sessão", "iniciar browser", "doctor", "ver abas", "screenshot cdp", or wants to verify the outer harness state.
inclusion: manual
evidence_level: curated
last_verified: 2026-09-29
---

# Skill: Antigravity Operator (`agyo`)

O `agyo` é o **Outer Harness** e Runtime Operacional para o Google Antigravity.
Ele opera como o guardião determinístico de ambiente, memória de sessão e ciclo de vida do navegador.

## Quando usar

- Quando iniciar uma nova sessão de trabalho ou projeto (`agyo init`)
- Para verificar diagnósticos de máquina, Git, Chrome, Antigravity e chaves API (`agyo doctor`)
- Para consultar o objetivo atual e progresso das tarefas (`agyo session status`)
- Para acompanhar o raciocínio do modelo e notificações em tempo real (`agyo session watch`)
- Para arquivar uma sessão concluída e reiniciar o ciclo (`agyo session archive`)
- Para iniciar o Chrome em perfil isolado sem sujar o perfil pessoal (`agyo browser start`)
- Para inspecionar abas, executar JavaScript ou tirar screenshot via CDP sem bibliotecas externas (`agyo browser tabs`, `eval`, `shot`)
- Para instalar a proteção de continuidade no Git (`agyo hook install`)

---

## Comandos Operacionais

### 1. Diagnóstico de Host e Dependências
```bash
agyo doctor
```
Verifica sistema operacional, processos do Google Antigravity, variáveis de display (X11/Wayland/Headless), binário do Chrome, porta DevTools 9222, Git, NPX e Gemini API Keys (BYOK).

### 2. Memória Operacional de Sessão & Streaming em Tempo Real (`.agents/session/`)
```bash
# Inicializar memória no repositório atual:
agyo init

# Verificar status da sessão ativa (objetivo, fase, métricas):
agyo session status

# Acompanhar raciocínio e ferramentas da IA em tempo real com notificações nativas no desktop:
agyo session watch

# Acompanhar apenas os últimos 5 passos sem streaming contínuo:
agyo session watch --once --steps 5

# Arquivar sessão concluída no histórico (.agents/session/archive/):
agyo session archive
```

### 3. Navegador Isolado, Multi-Profile Intelligence & CDP
```bash
# Descobrir e classificar perfis do Chrome (personal, workspace, ghost):
agyo browser profiles

# Resolver qual perfil autenticado e URL (?authuser=) usar para um domínio/URL:
agyo browser resolve <url-ou-dominio>

# Iniciar Chrome isolado na porta 9222 (com seletor agnóstico de perfil opcional):
agyo browser start
agyo browser start --profile workspace

# Forçar modo headless explicitamente:
agyo browser start --headless

# Verificar status do Chrome, perfil ativo e PID:
agyo browser status

# Listar abas abertas e IDs:
agyo browser tabs

# Abrir ou reutilizar aba existente/em branco na URL:
agyo browser open <url>

# Fechar aba por ID:
agyo browser close <targetId>

# Executar JavaScript na aba ativa:
agyo browser eval "document.title"

# Capturar screenshot PNG da aba ativa via CDP:
agyo browser shot screenshot.png

# Encerrar graciosamente o processo do Chrome:
agyo browser stop
```

### 4. Integração Nativa com Google NotebookLM
```bash
# Verificar status de conexão e autenticação no Google NotebookLM:
agyo notebooklm status

# Abrir o Google NotebookLM no Chrome isolado:
agyo notebooklm open

# Listar cadernos disponíveis na conta:
agyo notebooklm list

# Consultar fontes e fazer perguntas fundamentadas a um caderno:
agyo notebooklm ask <notebook-id> "<pergunta>"

# Adicionar arquivos ou notas locais como fontes em um caderno:
agyo notebooklm push <notebook-id> <arquivo-ou-texto>

# Executar como servidor MCP stdio para o Antigravity / Cursor / Claude:
agyo notebooklm mcp
```
*(Aliases curtos suportados: `agyo notebook` e `agyo nblm`)*

### 5. Computer Use (Automação de Desktop & Accessibility Tree)
```bash
# Inspecionar a árvore semântica da janela ativa (rápido, ~200 tokens de texto, 0 imagens):
agyo computer inspect

# Inspecionar em formato JSON completo:
agyo computer inspect --json

# Capturar print otimizado em JPEG com crop na janela ativa (fallback visual):
agyo computer shot --window [arquivo.jpg]

# Clicar em elemento pelo ID de referência ou por coordenadas (x y):
agyo computer click ref_2
agyo computer click 450 320

# Digitar texto no elemento em foco:
agyo computer type "texto a ser digitado"

# Acionar atalho de teclado ou tecla especial:
agyo computer key "cmd+space"
agyo computer key "return"

# Executar macro sequencial em lote:
agyo computer batch '[{"action":"click","ref":"ref_1"},{"action":"type","text":"busca"},{"action":"key","key":"return"}]'

# Iniciar servidor Model Context Protocol (MCP) stdio para agentes:
agyo computer mcp
```

### 6. Proteção de Continuidade Git (Pre-Commit Sensor)
```bash
# Instalar o hook de pre-commit no repositório:
agyo hook install

# Remover o hook:
agyo hook uninstall
```

### 7. Sincronização de Regras e Manifestos
```bash
agyo sync
```
Instala a regra global `session-agent.md`, os servidores MCP canônicos e a skill `agyo` no ambiente local.
