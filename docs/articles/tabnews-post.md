# Operação ao redor do Coding Agent: por que dar ferramentas para a IA não basta

Enquanto construía o [Antigravity Operator (`agyo`)](https://github.com/tiagovilasboas/antigravity-operator), percebi que dar ferramentas a um coding agent não resolve sozinho a operação ao redor dele.

É fácil se impressionar vendo uma IA rodar comandos no terminal, inspecionar arquivos ou navegar na web. Mas quem já tentou colocar um coding agent para operar em projetos reais rapidamente esbarra em três problemas práticos:

1. **Amnésia operacional:** uma sessão nova perde facilmente o objetivo principal e as decisões de arquitetura tomadas vinte minutos atrás.
2. **Poluição de ambiente e janelas fantasmas:** o navegador abre no Chrome pessoal do desenvolvedor, misturando cookies de produção, ou abre dezenas de abas órfãs que devoram memória RAM. Em servidores Linux sem interface gráfica (CI ou VPS sem `$DISPLAY`), o processo simplesmente quebra se não receber as flags corretas.
3. **Conclusão precipitada sem verificação:** o modelo encerra a resposta afirmando que terminou o trabalho, mas deixou arquivos alterados sem rodar a suíte de testes e sem atualizar o checklist de tarefas.

É como entregar uma oficina mecânica de ponta para um profissional muito ágil, mas sem prancheta de bordo, sem checklist de entrega e com ferramentas espalhadas em gavetas diferentes a cada turno. Ele até consegue produzir, mas alguém precisa preparar o ambiente, anotar o que já foi acordado e garantir que os sensores de segurança estão ativos.

Foi para cuidar dessa fronteira determinística que escrevi o `agyo`, uma CLI local de código aberto em Go puro (`CGO_ENABLED=0`, sem dependências externas no `go.mod`), desenhada como um outer harness operacional para o ecossistema do Google Antigravity.

---

## O agente precisa de um entorno operacional

Um coding agent em produção é a soma de duas partes distintas: **modelo + harness**.

O fornecedor (Google, Anthropic, OpenAI) cuida do modelo de linguagem e da interface de chat. Mas o **user harness** (o contexto do host local, a persistência de decisões no disco e a validação do código produzido) continua sendo nossa responsabilidade como engenheiros de software.

No `agyo`, a máquina de estados não depende de bancos de dados nem de runtimes pesados em Python ou Node.js. O comando `agyo init` cria três arquivos simples em Markdown no projeto:

```text
.agents/session/
├── state.md       # objetivo atual, fase e bloqueios
├── decisions.md   # decisões de arquitetura e premissas acordadas
└── todo.md        # tarefas concluídas, ativas e pendentes
```

O modelo e o desenvolvedor atualizam esses arquivos conforme o raciocínio progride. Se a janela de contexto estourar ou se você fechar o terminal e voltar no dia seguinte, o estado do projeto permanece intacto no disco.

Para auditar o andamento:

```bash
$ agyo session status
📋 Active Session Overview (.agents/session/)
🎯 Objective : Autonomous OS Runtime & Harness for Google Antigravity
⚡ Phase     : Execution & Browser Validation (100% verified)
📊 Progress  : 4/4 tasks completed (100%)
```

Essa escolha por Markdown puro é deliberada: qualquer editor lê, o Git versiona sem ruído e a depuração é transparente. Mas persistir texto puro em disco não faz milagre sozinho se o modelo esquecer de atualizar os arquivos. É aqui que entra a engenharia de harness.

---

## Arquitetura Cognitiva BDI e o Outer Harness de Fowler

Para estruturar o comportamento do agente de forma determinística, o `agyo` adota a arquitetura cognitiva clássica **BDI (Belief-Desire-Intention)** combinada ao modelo de **Outer Harness (Guias e Sensores)** conceituado por Martin Fowler:

1. **Beliefs (Crenças - O que o agente percebe do mundo):**
   Representa o estado do workspace e do ambiente verificado computacionalmente. É mantido por arquivos auditáveis (`state.md`, `decisions.md`), RAG multimodal fundamentado sem alucinações no Google NotebookLM (`agyo notebooklm`), fronteiras de exclusão de arquivos (`.agentignore`) e diagnósticos do host via `agyo doctor`.
2. **Desires (Desejos - O que o sistema quer alcançar):**
   Representa o objetivo acordado e as invariantes invioláveis. O objetivo ativo da sessão, as especificações técnicas (`docs/plans/`), o orçamento estrito de tokens $O(1)$ e os guardrails operacionais (como o Guardrail de Janela Única).
3. **Intentions (Intenções - O que o agente se compromete a executar):**
   Representa a fila de ações concretas e compromissos firmados. É materializado pelo checklist de tarefas em `.agents/session/todo.md`, snapshots de segurança atômicos (`agyo checkpoint` e `agyo rollback`) e os gates de ciclo de vida nativos do Antigravity 2.0.

A tabela abaixo sintetiza como Guias (orientação prévia/feedforward) e Sensores (auditoria posterior/feedback) governam cada responsabilidade:

| Responsabilidade | Guia (Antes da Ação) | Sensor (Depois da Ação) |
|---|---|---|
| **Ciclo de Vida e Memória** | Injeção determinística de `state.md` e `todo.md` via hook `PreInvocation` | Bloqueio de saída com tarefas pendentes via hook `Stop` |
| **Integridade de Código** | Checkpoint automático de git no hook `PreToolUse` antes de comandos arriscados | Rastreio de mutações (`DirtyCode`) no hook `PostToolUse` liberado apenas por testes |
| **Ambiente e Host** | Diagnósticos de SO, binários, display X11/Wayland e porta 9222 | Sensor computacional do `agyo doctor` e auto-recuperação (`agyo doctor --fix`) |
| **Navegação Web e Abas** | Perfil dedicado em `~/.gemini/antigravity-browser-profile` com porta 9222 | Guardrail de Janela Única e cliente CDP WebSocket em Go puro |
| **RAG e FinOps de Tokens** | Grounding externo no Google NotebookLM (sem context stuffing) | Bridge CDP/MCP consulta notas e citações com 99,2% de economia de tokens |
| **Rotina de Estudos** | Templates pedagógicos embutidos (método Feynman, plano reverso de provas) | Comandos `agyo study` integrados com Google Calendar e Google Docs |

---

## Lifecycle Hooks nativos do Antigravity 2.0

Regras em texto puro ("por favor, lembre de rodar a suíte de testes antes de dizer que concluiu") falham com frequência quando a janela de contexto cresce. Agentes sofrem de viés de conclusão prematura: eles concluem a resposta assumindo que o código funciona.

No Antigravity 2.0, o fluxo de execução expõe pontos de interceptação nativos configurados via `hooks.json`. O `agyo` se conecta a esse ciclo sem scripts em Python nem ferramentas externas através do comando:

```bash
agyo hook antigravity install
```

O comando realiza um merge idempotente no arquivo `hooks.json` (do usuário ou do workspace), preservando outros plugins e criando um backup datado de segurança.

A partir desse momento, o `agyo` atua diretamente no loop cognitivo do agente:

* **`PreInvocation` (Memória determinística):** na primeira chamada ou ao alternar conversas, o hook lê `.agents/session/state.md` e `.agents/session/todo.md` e injeta um resumo executivo como mensagem efêmera (`ephemeralMessage`), com teto de 1,5 KB. O modelo inicia o raciocínio sabendo exatamente o objetivo e os próximos passos sem depender de prompt manual.
* **`PreToolUse` (Gate de segurança e auto-checkpoint):** intercepta a chamada de ferramentas antes da execução. Aplica portão de permissão (`allow` / `deny`) e, antes de comandos que alteram o disco de forma destrutiva, gera automaticamente um checkpoint de segurança no Git.
* **`PostToolUse` (Sensor de código sujo):** se o agente usar ferramentas como `write_to_file` ou `replace_file_content` em arquivos de código, o hook marca o estado da sessão como `DirtyCode = true`. Essa marcação só é limpa quando um comando de teste ou compilação (`go test`, `npm test`, `pytest`, `tsc`) for executado com sucesso.
* **`Stop` (Portão de conclusão rigoroso):** quando o modelo decide parar (`terminationReason: model_stop`), o hook avalia duas condições:
  1. Existe alguma tarefa marcada como pendente em `## Em Progresso` no `todo.md`?
  2. Houve alteração de código que ainda não foi validada por testes (`DirtyCode == true`)?
  Se qualquer uma for verdadeira, o hook retorna `decision: continue` com a explicação do que falta. O agente é forçado a continuar e validar seu trabalho. Para evitar travamentos em casos anômalos, há um mecanismo anti-loop que permite o encerramento após duas recusas consecutivas.

Também mantemos o clássico hook de Git pre-commit (`agyo hook install`), garantindo que commits manuais ou automáticos alertem caso a documentação da sessão esteja desatualizada.

---

## Guardrail de Janela Única e cliente CDP WebSocket em Go puro

Quando um coding agent precisa validar uma interface web ou interagir com um serviço, abrir o Chrome padrão do usuário traz riscos: cookies de contas pessoais misturados e abas de produção expostas. Pior ainda: muitos agentes abrem uma nova aba ou janela a cada interação, acumulando dezenas de instâncias órfãs que esgotam a memória RAM da máquina de desenvolvimento.

Para resolver isso, o `agyo` traz uma arquitetura de supervisão de navegador:

```bash
agyo browser start
```

Esse comando sobe uma instância dedicada do Chrome com diretório de perfil isolado (`~/.gemini/antigravity-browser-profile`) e escuta na porta `9222`:
* No macOS e Linux com ambiente gráfico, abre a janela visível para você acompanhar a navegação.
* Em servidores Linux sem display (`$DISPLAY` vazio), ativa automaticamente `--headless=new`, `--no-sandbox` e flags de memória compartilhada.

### O cliente CDP em Go puro

Eliminamos totalmente dependências de Node.js, Puppeteer ou Playwright para controlar o navegador. O `agyo` implementa internamente um cliente do Chrome DevTools Protocol (CDP) usando a biblioteca padrão do Go (`net/http` e RFC 6455 para WebSocket):
* Handshake WebSocket manual sobre conexão TCP limpa;
* Máscara de frames client-to-server conforme especificação;
* Timeout de conexão de 15 segundos com descarte seletivo de eventos assíncronos e frames de controle (ping/pong).

### O Guardrail de Janela Única

Para evitar a proliferação desordenada de abas, implementamos o **Guardrail de Janela Única**:
Ao executar comandos de navegação ou autenticação, o `agyo` consulta o endpoint `/json/list` do DevTools. Se já existir uma aba aberta para o domínio ou URL requisitado, ele ativa e reutiliza essa mesma aba em vez de abrir outra.

Os comandos da CLI operam de forma direta:

```bash
agyo browser tabs                    # Lista abas abertas e target IDs via CDP
agyo browser open https://github.com # Abre ou reutiliza aba existente (Janela Única)
agyo browser eval "document.title"   # Executa JavaScript na aba ativa via CDP
agyo browser shot tela.png           # Captura screenshot PNG diretamente via protocolo
agyo browser stop                    # Encerra o Chrome isolado com sinal gracioso
```

---

## FinOps de contexto: economizando de 97% a 99,2% de tokens com NotebookLM

Quem já tentou usar um coding agent para estudar uma nova biblioteca, analisar uma documentação extensa ou estudar uma palestra técnica longa conhece o problema do *context stuffing*: colar uma transcrição de 1 hora (~20.000 tokens) diretamente na janela de conversa acumula mais de 400.000 tokens ao longo de 15 turnos. Em pesquisas de contexto denso envolvendo capítulos inteiros de livros ou múltiplos módulos de documentação (~100.000 tokens), esse histórico acumulado passa facilmente de 1,5 milhão de tokens trafegados. Além de estourar limites de taxa por minuto (TPM), o modelo sofre de perda de atenção no meio do contexto (*lost in the middle*).

Para resolver isso de forma pragmática, o `agyo` conecta o ecossistema com o Google NotebookLM dividindo as responsabilidades:

1. **Cérebro de Pesquisa (NotebookLM):** o acervo pesado (vídeos do YouTube, PDFs ou artigos) é indexado no NotebookLM via automação CDP ou ferramentas MCP embutidas. A indexação multimodal e a recuperação rodam no Gemini 1.5 Pro na infraestrutura do Google, a custo zero de cota no Antigravity.
2. **Mãos de Execução (Antigravity):** o agente faz apenas consultas cirúrgicas (~60 tokens) e recebe respostas sintetizadas com citações precisas (~800 tokens por turno).

O volume total de tokens trafegados na janela ativa ao longo de 15 turnos cai para apenas cerca de 12.000 tokens: uma **economia modelada de 97% na análise de videoaulas e de até 99,2% em pesquisas de contexto denso**, mantendo alta fidelidade e citações para os trechos de origem.

---

## Student Edition: o fluxo contínuo do `agyo study`

Estendemos esse harness para apoiar estudantes universitários, pesquisadores e engenheiros em rotina de aprendizado contínuo através do comando `agyo study`:

```bash
agyo study <subcommand> [argumentos]
```

O fluxo acadêmico é dividido em três etapas práticas:

### 1. Ingestão e Síntese Feynman (`agyo study ingest`)

```bash
agyo study ingest "https://youtu.be/exemplo" --notebook <id-caderno> --title "Algoritmos Distribuidos"
```

O comando envia o conteúdo para o caderno do NotebookLM, solicita uma síntese pedagógica baseada no método Feynman e grava dois artefatos na pasta `estudos/<slug>/`:
* `artigo.md`: resumo conceitual com termos-chave, explicações didáticas e citações das fontes.
* `flashcards.md`: cartões no formato pergunta e resposta prontos para importação no Anki ou Obsidian.

### 2. Cronograma Reverso de Provas (`agyo study exam-prep`)

```bash
agyo study exam-prep --disciplina "Sistemas Operacionais" --data 2026-11-20 --dias 7
```

Calcula automaticamente um cronograma reverso com cinco marcos de estudo espaçado (fundamentos, resolução de exercícios, simulado, revisão de flashcards e dia da avaliação). As tarefas são registradas em `.agents/session/todo.md` e, se a autenticação estiver ativa, os blocos de estudo são sincronizados diretamente na agenda do Google Calendar.

### 3. Simulado Interativo Feynman (`agyo study quiz`)

```bash
agyo study quiz --notebook <id-caderno> --topic "Gerenciamento de Memoria"
```

Inicia um checkpoint interativo direto no terminal. O NotebookLM formula uma questão desafiadora de múltipla escolha estritamente baseada nas fontes do caderno. Após a resposta do estudante, o tutor Feynman avalia a resposta: se estiver correta, aprofunda o mecanismo conceitual; se estiver incorreta, explica com clareza onde esteve a falha de raciocínio.

---

## Google Workspace Bridge em Go puro (`agyo google`)

Para integrar a agenda e os relatórios sem exigir dependências como o `gcloud` CLI ou bibliotecas de terceiros, construímos a **Google Workspace Bridge**:

* **OAuth 2.0 Loopback nativo (RFC 8252):** autenticação local em porta efêmera (`127.0.0.1:8085`). O fluxo reutiliza a aba ativa do Chrome isolado se ele estiver em execução (obedecendo ao Guardrail de Janela Única) e salva as credenciais com permissão restrita `0600`.
* **Google Calendar API v3 em Go puro:** listagem e criação de eventos com suporte a múltiplos contextos (pessoal, faculdade ou trabalho).
* **Google Docs API v1 em Go puro:** leitura de documentos com extração de texto limpo e criação de novos documentos formatados.
* **Servidor MCP stdio (`agyo google mcp`):** expõe as ferramentas de calendário e documentos via Model Context Protocol para que o agente no Antigravity possa agendar revisões ou redigir documentação de forma autônoma.

Comandos para o desenvolvedor usar no terminal:

```bash
agyo google auth                                                     # Autentica via loopback local seguro
agyo google status                                                   # Verifica validade e expiracao do token
agyo google calendar list --today                                    # Lista compromissos do dia
agyo google calendar add "Revisao Raft" --start "2026-10-18T19:00:00Z" # Agenda bloco de estudo
agyo google docs create "Relatorio de Entrega" --content "Resumo..." # Cria documento no Docs
agyo google docs read <documentId-ou-url>                           # Le conteudo estruturado do Doc
```

---

## Por que Go e binário único?

A CLI inteira foi construída com **zero dependências externas no `go.mod`**.

Tudo o que o `agyo` executa roda em cima dos pacotes nativos do Go (`net/http`, `os/exec`, `encoding/json` e `embed`). Os templates de regras, os manifestos de MCPs e as skills pedagógicas (`video-to-article`, `feynman-code-tutor`, `student-study-planner`, `token-budget-guard`) ficam embutidos no binário compilado.

O resultado é um executável único e estático de ~14 MB:
* Não precisa de `npm install`, `pip`, Docker ou Node.js instalados na máquina do desenvolvedor;
* Roda sem permissões de `sudo` ou root em ambientes Linux acadêmicos e corporativos;
* Suporta macOS, Linux e Windows (via WSL ou compilação nativa).

---

## O que isso NÃO resolve (honestidade técnica)

Manter um olhar realista sobre as limitações da ferramenta é fundamental:
* O `agyo` não é uma sandbox de isolamento total contra código hostil: em servidores headless, o Chrome utiliza flags permissivas para operar em contêineres, o que exige ambiente confiável.
* Uma CLI e um harness determinístico não corrigem falhas de raciocínio lógico intrínsecas do modelo: eles apenas impedem que o modelo ignore o ambiente, esqueça o contexto e termine sem rodar os testes.
* A supervisão e o julgamento arquitetural continuam sendo responsabilidade humana.

O projeto é 100% de código aberto sob licença MIT:
👉 **[github.com/tiagovilasboas/antigravity-operator](https://github.com/tiagovilasboas/antigravity-operator)**

Se você já utiliza coding agents no seu dia a dia de engenharia ou estudos: qual tem sido o seu maior desafio prático? Manter a continuidade de contexto entre sessões, isolar o navegador de testes ou garantir que o agente não encerre tarefas sem validar o código?
