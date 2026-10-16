# Post LinkedIn: Antigravity Operator (agyo)

Dar ferramentas para um coding agent não resolve sozinho a operação ao redor dele.

É fácil se impressionar vendo uma IA rodar comandos no terminal ou editar arquivos. Mas quem já tentou colocar um coding agent para operar em projetos reais rapidamente esbarra em 3 problemas práticos:

1. **Perda de continuidade:** Uma sessão nova esquece o objetivo principal e as decisões de arquitetura tomadas 10 minutos atrás.
2. **Poluição de ambiente:** O agente abre o navegador usando seu Chrome pessoal com abas pessoais e cookies de produção misturados.
3. **Quebra silenciosa em servidores:** Em máquinas Linux de CI, VPS ou Docker (sem tela/display gráfico), o navegador tenta abrir uma janela física e simplesmente crasha.

Foi para resolver essa fronteira determinística que criei o **Antigravity Operator (`agyo`)**, um runtime operacional e outer harness de código aberto escrito em **Go puro (sem CGO e com zero dependências externas)**.

Inspirado no modelo de Harness Engineering do Martin Fowler, o `agyo` divide a responsabilidade entre **Guias** (que orientam o modelo antes do trabalho) e **Sensores** (que auditam a máquina e o código depois):

🔹 **Memória Operacional de Sessão (`agyo init` / `session status`):**
Persistência simples e auditável em Markdown (`.agents/session/state.md`, `decisions.md` e `todo.md`). O estado sobrevive a quedas de conexão ou janelas de contexto estouradas.

🔹 **Supervisor de Navegador e Protocolo CDP (`agyo browser`):**
Inicia uma instância isolada do Chrome na porta 9222 com profile dedicado. No Mac/desktop abre a janela visual; em servidores Linux sem tela ativa modo Headless automaticamente. Inclui um cliente nativo do Chrome DevTools Protocol em Go puro para inspecionar abas, executar JavaScript e capturar screenshots sem precisar de Node.js ou Python.

🔹 **FinOps de Tokens e Student Edition (`agyo study`):**
Redução medida de 99,2% no consumo de tokens em tarefas de pesquisa e estudo. Em vez de entupir o prompt com transcrições de 1 hora (~20.000 tokens) que acumulam 1,5 milhão de tokens ao longo de 15 turnos, o `agyo` ancora o material denso no Google NotebookLM via CDP e MCP embutido. O agente faz apenas consultas cirúrgicas com citações (~12.000 tokens ativos no total), economizando cota e evitando alucinações.

🔹 **Google Workspace Bridge (Calendar + Docs):**
Ponte nativa com OAuth 2.0 loopback em Go puro para agendar blocos de estudo no Google Calendar e exportar relatórios de revisão diretamente no Google Docs, fechando o fluxo acadêmico contínuo sem nenhuma biblioteca externa.

🔹 **Git Pre-Commit Hook (`agyo hook install`):**
Um sensor que protege o fluxo de trabalho: se o desenvolvedor ou o agente tentarem commitar código sem atualizar o objetivo ou o progresso da sessão, o hook alerta antes do commit ser gravado.

🔹 **Diagnóstico de Host (`agyo doctor`):**
Audita em segundos se a máquina está pronta: Git, Chrome, porta DevTools 9222, variáveis de display gráfico, runtimes MCP e credenciais do Google Workspace.

O projeto é 100% open source e focado no ecossistema do Google Antigravity e desenvolvedores Google AI.

👉 **Repositório no GitHub:** https://github.com/tiagovilasboas/antigravity-operator  
👉 **Artigo completo com o manifesto de arquitetura:** [link na documentação / primeiro comentário]

Para quem já está usando coding agents no dia a dia: qual parte da operação mais consome seu tempo hoje? Memória entre sessões, isolamento de ferramentas ou estabilidade entre máquinas diferentes?

#EngenhariaDeSoftware #Golang #ArtificialIntelligence #CodingAgents #GoogleAntigravity #DevOps #SystemDesign #OpenSource
