# Post LinkedIn: Antigravity Operator (agyo)

Dar ferramentas para um coding agent não resolve sozinho a operação ao redor dele.

É fácil se impressionar vendo uma IA rodar comandos no terminal ou editar arquivos. Mas quem já tentou colocar um coding agent para operar em projetos reais de engenharia rapidamente esbarra em três problemas práticos:

1. **Amnésia operacional:** uma sessão nova esquece o objetivo principal e as decisões de arquitetura tomadas vinte minutos atrás.
2. **Poluição de ambiente e janelas fantasmas:** o agente abre o Chrome pessoal misturando cookies e credenciais, ou acumula dezenas de abas órfãs que devoram memória RAM. Em servidores Linux sem tela (CI ou VPS sem `$DISPLAY`), o processo quebra.
3. **Conclusão precipitada sem testes:** o agente avisa no chat que terminou a tarefa, mas deixou código alterado sem rodar testes e sem atualizar o checklist de progresso.

Para cuidar dessa fronteira determinística, construí o **Antigravity Operator (`agyo`)**, um runtime operacional e outer harness de código aberto escrito em **Go puro (CGO_ENABLED=0 e zero dependências externas no go.mod)** para o ecossistema do Google Antigravity e desenvolvedores Google AI.

Inspirado na arquitetura cognitiva **BDI (Belief-Desire-Intention)** e no modelo de **Harness Engineering (Guias e Sensores)** de Martin Fowler, o `agyo` estrutura a operação do agente em pilares práticos:

🔹 **Arquitetura Cognitiva BDI (Beliefs, Desires, Intentions):**
* **Beliefs (Crenças):** percepção auditável do workspace em Markdown (`.agents/session/state.md`, `decisions.md`), RAG multimodal fundamentado via Google NotebookLM e diagnóstico do host com `agyo doctor`.
* **Desires (Desejos):** objetivo da sessão com garantia de zero regressões, orçamento estrito de tokens O(1) e guardrails de segurança.
* **Intentions (Intenções):** checklist acionável em `.agents/session/todo.md`, snapshots de segurança atômicos (`agyo checkpoint` / `rollback`) e interceptação de ciclo de vida.

🔹 **Lifecycle Hooks nativos do Antigravity 2.0 (`agyo hook antigravity install`):**
Conecta a CLI diretamente ao loop do agente através do `hooks.json` da IDE:
* `PreInvocation`: injeta deterministicamente o objetivo e tarefas ativas como mensagem efêmera no início da conversa (teto de 1,5 KB).
* `PreToolUse`: bloqueia comandos destrutivos e gera auto-checkpoint no Git antes de mutações arriscadas.
* `PostToolUse`: sensor que marca o código como sujo (`DirtyCode = true`) após edições em arquivos de projeto.
* `Stop`: portão rigoroso que impede o agente de encerrar a resposta se houver tarefas em progresso ou código alterado sem testes validados (`go test`, `npm test`, `pytest`).

🔹 **Guardrail de Janela Única e cliente CDP WebSocket em Go puro (`agyo browser`):**
Implementamos o Chrome DevTools Protocol nativo sobre WebSockets (RFC 6455) usando apenas a biblioteca padrão do Go, sem Node.js, Puppeteer ou Python:
* Inicia o Chrome com perfil isolado na porta 9222 (`agyo browser start`), ativando headless automaticamente em servidores sem display.
* O **Guardrail de Janela Única** detecta abas abertas via CDP e reutiliza a aba ativa antes de abrir novas páginas, eliminando o acúmulo de abas zumbis.
* Comandos diretos: `agyo browser tabs`, `agyo browser open <url>`, `agyo browser eval "<js>"` e `agyo browser shot <arquivo.png>`.

🔹 **FinOps de Tokens (de 97% a 99,2% de economia de cota):**
Separamos o trabalho entre Cérebro (NotebookLM) e Mãos (Antigravity). Em vez de entupir o prompt com transcrições de videoaulas (~20.000 tokens) ou materiais densos de pesquisa (~100.000 tokens) que acumulam de 400 mil a mais de 1,5 milhão de tokens ao longo de 15 turnos, o material bruto é indexado no NotebookLM a custo zero de cota. O agente faz apenas consultas cirúrgicas com citações de fontes (~12.000 tokens no total). Economia e eliminação de alucinações comprovadas em cenário real.

🔹 **Student Edition (`agyo study`):**
Fluxo contínuo para estudantes, pesquisadores e engenheiros:
* `agyo study ingest <url> --notebook <id> [--title <titulo>]`: indexa aulas ou artigos, gerando guia didático no método Feynman e flashcards para Anki/Obsidian em `estudos/<slug>/`.
* `agyo study exam-prep --disciplina "<nome>" --data <YYYY-MM-DD> [--dias 7]`: cronograma de preparação reversa com marcos de estudo espaçado, registro em `todo.md` e agendamento automático no Google Calendar.
* `agyo study quiz --notebook <id> [--topic <tema>]`: simulado interativo no terminal onde o tutor Feynman gera perguntas desafiadoras baseadas nas fontes do caderno e avalia as respostas em tempo real.

🔹 **Google Workspace Bridge em Go puro (`agyo google`):**
Ponte nativa com OAuth 2.0 loopback (RFC 8252 na porta 8085) sem exigir gcloud CLI ou bibliotecas externas:
* Gerencia eventos do Google Calendar v3 (`agyo google calendar list` e `add`).
* Lê e cria documentos no Google Docs v1 (`agyo google docs read` e `create`).
* Servidor MCP stdio (`agyo google mcp`) para que o agente agende reuniões ou exporte relatórios automaticamente.

🔹 **Sensores de Host e Auto-Cura (`agyo doctor`):**
Audita em segundos o ambiente: Git, Chrome, porta 9222, variáveis de display, hooks e credenciais do Google Workspace. O comando `agyo doctor --fix` repara automaticamente arquivos de sessão corrompidos ou ausentes.

O projeto é 100% open source (licença MIT), em binário único compilado estaticamente para macOS, Linux e Windows (WSL).

👉 **Código no GitHub:** https://github.com/tiagovilasboas/antigravity-operator  
👉 **Artigo técnico detalhado:** [link na documentação / comentários]

Para quem já está usando coding agents no fluxo diário: qual tem sido o maior gargalo no seu time? Amnésia entre sessões, isolamento de ferramentas ou garantir que a IA não termine tarefas sem validar testes?

#EngenhariaDeSoftware #Golang #ArtificialIntelligence #CodingAgents #GoogleAntigravity #DevOps #SystemDesign #OpenSource #SoftwareArchitecture
