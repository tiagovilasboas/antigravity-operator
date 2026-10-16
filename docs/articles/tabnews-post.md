# Operação ao redor do Coding Agent: por que dar ferramentas para a IA não basta

Enquanto construía o [Antigravity Operator (`agyo`)](https://github.com/tiagovilasboas/antigravity-operator), percebi que dar ferramentas a um coding agent não resolve sozinho a operação ao redor dele. 

Uma sessão nova perde facilmente o objetivo e as decisões anteriores; o Chrome aberto pelo agente precisa de um perfil separado para não vazar cookies pessoais; e uma máquina Linux em CI ou VPS sem tela (`$DISPLAY`) exige flags específicas para o navegador não quebrar.

É como entregar uma oficina mecânica de ponta para uma pessoa muito rápida, mas sem prancheta de bordo e com ferramentas guardadas em gavetas diferentes em cada turno. Ela até consegue produzir, mas alguém precisa preparar o ambiente, anotar o que já foi decidido e garantir que os sensores estão ligados. 

Foi para cuidar dessa fronteira determinística que escrevi o `agyo`, uma CLI local em Go puro (`CGO_ENABLED=0`) sem nenhuma dependência externa.

---

## O agente precisa de um entorno operacional

Um coding agent na prática é a soma de duas coisas: **modelo + harness**. 

O fornecedor (Google, Anthropic, OpenAI) cuida do modelo e da interface de chat. Mas o **user harness** (o contexto da sua máquina, a persistência de decisões no disco e a validação do código gerado) continua sendo nossa responsabilidade como engenheiros.

No `agyo`, decidi que a máquina de estados não precisava de banco de dados nem de runtime pesado em Python ou Node. Um comando `agyo init` cria três arquivos simples em Markdown no projeto:

```text
.agents/session/
├── state.md       # objetivo atual, fase e bloqueios
├── decisions.md   # decisões de arquitetura e premissas acordadas
└── todo.md        # tarefas concluídas, ativas e pendentes
```

O modelo atualiza esses arquivos conforme navega no código. Se a janela de contexto estourar ou se eu fechar o terminal e voltar amanhã, o estado continua lá. 

Para inspecionar:
```bash
$ agyo session status
📋 Active Session Overview (.agents/session/)
🎯 Objective : Autonomous OS Runtime & Harness for Google Antigravity
⚡ Phase     : Execution & Browser Validation (100% verified)
📊 Progress  : 4/4 tasks completed (100%)
```

Essa escolha por Markdown é deliberada: qualquer editor lê, o Git versiona (se o time quiser) e a depuração é transparente. Mas persistir texto não faz milagre sozinho: o modelo ainda pode alucinar ou esquecer de preencher os checkboxes. Por isso o `status` atua como sensor computacional, auditando se a estrutura física está coerente.

---

## O ciclo de Guia e Sensor (Fowler Outer Harness)

Inspirado no modelo de *Harness Engineering* do Martin Fowler, separei cada responsabilidade entre quem orienta antes (**guia**) e quem verifica depois (**sensor**):

| Responsabilidade | Guia (antes do trabalho) | Sensor (depois do trabalho) |
|---|---|---|
| **Continuidade de Sessão** | Templates embutidos de `state.md` e regras de sessão | `agyo session status` e git pre-commit hook (`agyo hook install`) |
| **Ambiente e Host** | Diagnósticos esperados de OS, Git, Chrome e NPX | `agyo doctor` checa binários, display X11/Wayland e porta 9222 |
| **Navegação Web** | Flag de perfil isolado em `~/.gemini/antigravity-browser-profile` | Pure-Go CDP client consulta `/json/list` e WebSocket na porta 9222 |
| **RAG e FinOps de Tokens** | Grounding externo no Google NotebookLM (sem context stuffing) | Bridge CDP/MCP consulta notas e fontes com ~99,2% de economia de cota |
| **Automação de Estudos** | Skills embutidas (`feynman-code-tutor`, `student-study-planner`) | `agyo study` orquestra ingestão, Google Calendar e Google Docs |
| **Qualidade do Operator** | Arquitetura por pacotes desacoplados (SRP, KISS, YAGNI) | Testes unitários com `-race` (>80% cobertura) e CI cross-platform |

---

## Navegador isolado e CDP em Go puro

Quando um agente precisa testar o front-end ou depurar chamadas de rede, abrir o Chrome pessoal é receita para vazamento de sessão ou interferência.

O `agyo browser start` sobe uma instância dedicada na porta `9222`:
* No macOS e Linux desktop, abre a janela visível para você ver o agente operando.
* No Linux de servidor (sem `$DISPLAY`), ativa automaticamente `--headless=new` e flags de memória compartilhada.

Recentemente eliminamos a necessidade de scripts rápidos em Node/Python para inspecionar o browser. Implementamos um cliente WebSocket (RFC 6455) na biblioteca padrão do Go para conversar com o Chrome DevTools Protocol:

```bash
agyo browser tabs                    # Lista abas ativas e IDs
agyo browser open https://github.com # Abre URL em nova aba
agyo browser eval "document.title"   # Executa JS na página ativa
agyo browser shot tela.png           # Tira screenshot PNG via CDP
agyo browser stop                    # Encerra graciosamente com SIGTERM
```

---

## FinOps de contexto: economizando 99,2% de tokens com NotebookLM

Quem já tentou usar um coding agent para estudar uma biblioteca nova ou analisar uma aula técnica longa conhece o problema do *context stuffing*: colar uma transcrição de 1 hora (~20.000 tokens) na janela de chat. Em uma conversa de 15 turnos, esse histórico acumulado passa facilmente de 1,5 milhão de tokens trafegados. Além de estourar limites de taxa, o modelo sofre com perda de foco no meio do prompt (*lost in the middle*).

Para resolver isso de forma pragmática, o `agyo` traz integração nativa com o Google NotebookLM:
1. O material bruto (PDFs, vídeos ou transcrições) é enviado para um caderno dedicado no NotebookLM via automação CDP ou MCP embutido.
2. A indexação pesada e o RAG multimodal rodam no Gemini 1.5 Pro dentro da infraestrutura do Google, a custo zero de cota no Antigravity.
3. O agente envia apenas perguntas cirúrgicas (~60 tokens) e recebe respostas fundamentadas com citações (~800 tokens).

O tráfego total acumulado ao longo da sessão cai de ~1.500.000 para ~12.000 tokens: uma **redução medida de 99,2% no consumo de tokens do prompt**.

---

## Student Edition: o fluxo contínuo do `agyo study`

Com o subcomando `agyo study`, estendemos esse harness para além da programação, conectando o fluxo de estudos de ponta a ponta:

```bash
agyo study ingest "https://youtu.be/..." --notebook <id>  # Indexa aula ou artigo no NotebookLM
agyo study exam-prep --notebook <id>                      # Gera guia e agenda blocos no Calendar
agyo study quiz --notebook <id>                           # Simulado interativo e relatório no Docs
```

Para integrar a agenda e os relatórios, implementamos o **Google Workspace Bridge**:
* **OAuth 2.0 Loopback nativo em Go puro (RFC 8252):** o usuário autentica no navegador uma única vez, recebendo o token de forma segura em porta local efêmera, sem exigir Node.js ou gcloud CLI instalado.
* **Ferramentas MCP para Google Calendar e Google Docs:** o agente consegue consultar e criar eventos de revisão espaçada na agenda, bem como redigir cadernos de revisão formatados no Docs.

---

## Por que Go e binário único?

A CLI foi construída com **zero dependências externas no `go.mod`**. 

Tudo o que o `agyo` faz roda em cima do `net/http`, `os/exec`, `embed` e da biblioteca padrão. Os templates de regras, MCPs e skills ficam embutidos no binário compilado. Se você baixar o binário de 6 MB em qualquer máquina Linux ou macOS, ele funciona na hora sem precisar de `npm install`, sem virtualenv e sem CGO.

---

## O que isso NÃO resolve (honestidade técnica)

O `agyo` não é um sandbox de segurança contra código hostil: o Chrome em headless recebe flags como `--no-sandbox` para rodar em containers, o que exige ambiente controlado. Além disso, uma CLI não substitui o julgamento humano: ela apenas garante que o agente e o desenvolvedor trabalham no mesmo ambiente previsível.

O código está 100% aberto no GitHub:
👉 **[github.com/tiagovilasboas/antigravity-operator](https://github.com/tiagovilasboas/antigravity-operator)**

Se você já começou a usar coding agents no seu fluxo diário, qual parte mais te dá dor de cabeça: perder o contexto entre sessões, isolar o navegador ou manter o ambiente configurado entre máquinas diferentes?
