# Operação ao redor do Coding Agent: Por que dar ferramentas não basta

> *Artigo de concepção e manifesto técnico do [Antigravity Operator (`agyo`)](https://github.com/tiagovilasboas/antigravity-operator).*  
> *Autor: Tiago de Carvalho Vilas Boas ([@tiagovilasboas](https://github.com/tiagovilasboas))*

---

Enquanto construía o Antigravity Operator, percebi que dar ferramentas a um coding agent não resolve sozinho a operação ao redor dele. Uma sessão nova pode perder o objetivo e as decisões anteriores; o Chrome precisa de um perfil separado; e uma máquina Linux sem interface gráfica exige outro modo de iniciar o navegador.

É como entregar um laboratório para uma pessoa muito rápida, mas sem caderno de bordo e com equipamentos diferentes em cada sala. Ela ainda consegue trabalhar, mas alguém precisa preparar o ambiente, registrar o que já foi decidido e conferir se os instrumentos estão prontos. Foi para cuidar dessa parte que comecei o [`agyo`](https://github.com/tiagovilasboas/antigravity-operator), uma CLI local escrita em Go.

O `agyo` não é outro modelo nem tenta substituir o Antigravity. Ele reúne algumas rotinas operacionais: cria arquivos de sessão no projeto, verifica dependências da máquina, sincroniza regras e manifestos MCP, e inicia ou encerra uma instância separada do Chrome.

---

## O agente precisa de um entorno operacional

Um coding agent é modelo + harness. O produto do fornecedor oferece modelo, ferramentas e orquestração; o user harness do projeto acrescenta contexto, limites, verificações e operação local.

No Antigravity Operator, a CLI faz a parte determinística que eu não quero reconstruir manualmente a cada sessão. O comando `agyo init` prepara três arquivos Markdown em `.agents/session/`:

```text
.agents/session/
├── state.md       # objetivo e situação atual
├── decisions.md   # decisões e trade-offs
└── todo.md        # tarefas em andamento e pendentes
```

O agente pode atualizar esse registro conforme trabalha. Depois, `agyo session status` resume o objetivo e as tarefas marcadas; `agyo session archive` guarda um resumo e prepara alguns arquivos para a próxima sessão. A implementação está em [`internal/session`](https://github.com/tiagovilasboas/antigravity-operator/tree/main/internal/session).

Essa escolha é deliberadamente simples: Markdown é legível, editável e pode ser inspecionado sem abrir uma ferramenta proprietária. Mas persistir texto não garante que o modelo o atualize corretamente nem que retome o trabalho sem erro. O `status` verifica campos e checkboxes que consegue interpretar; não avalia se as decisões registradas estão corretas ou completas.

---

## O que a CLI controla

O projeto separa a operação em comandos pequenos:

```bash
agyo doctor                 # verifica ferramentas e integrações locais
agyo init                   # prepara a memória de sessão no projeto atual
agyo session status         # resume o estado persistido
agyo browser start          # inicia Chrome com perfil separado
agyo browser start --headless
agyo browser status
agyo browser stop
agyo sync                   # instala regras e manifesto MCP padrão
```

O código Go cuida da parte que pode ser tratada por regras explícitas: detectar o sistema operacional, localizar Chrome ou Chromium, criar diretórios, salvar o PID do processo e consultar o endpoint local do Chrome DevTools Protocol. Os templates embutidos levam as regras e o manifesto junto do binário, em vez de exigir que o usuário encontre esses arquivos em outro checkout. O ciclo do navegador está em [`internal/profile`](https://github.com/tiagovilasboas/antigravity-operator/tree/main/internal/profile).

O objetivo é reduzir a configuração repetida. A CLI não interpreta a intenção do usuário, não decide se uma mudança de código está correta e não força o modelo a seguir as instruções. Essas decisões continuam no agente e na revisão humana.

---

## Guia e sensor: o ciclo que tentei fechar

O desenho segue a ideia de que cada controle precisa orientar antes da geração e verificar depois. No vocabulário de harness engineering, isso é **guia** e **sensor**.

| Preocupação | Guia antes do trabalho | Sensor depois do trabalho | Tipo e eixo |
|---|---|---|---|
| **Continuidade da tarefa** | Templates de `state.md`, `decisions.md` e `todo.md` + regra de atualização | `session status` lê objetivo e tarefas com formato conhecido | Guia inferencial; sensor computacional parcial; behaviour |
| **Ambiente local** | Instruções de setup e convenções do projeto | `agyo doctor` verifica Git, Chrome, NPX, endpoint DevTools e caminho do harness | Guia inferencial; sensor computacional; maintainability |
| **Navegador** | Regra para usar o perfil dedicado | Verificação do endpoint CDP e do estado do processo | Guia inferencial; sensor computacional; behaviour |
| **Qualidade do próprio `agyo`** | `AGENTS.md`, arquitetura por pacote e checklist de contribuição | Go tests, `go vet`, build, smoke test e cross-compilation no CI | Guia inferencial; sensor computacional; maintainability / architecture fitness |

Esse mapa também mostra os limites. O sensor do estado de sessão verifica uma representação; não consegue saber se o modelo realmente preservou o raciocínio importante. O `doctor` informa se alguns pré-requisitos respondem; não prova que um fluxo de agente vai concluir a tarefa. E os testes do `agyo` verificam o código do operador, não o comportamento de todo modelo que alguém conectar a ele.

O [CI do repositório](https://github.com/tiagovilasboas/antigravity-operator/blob/main/.github/workflows/ci.yml) declara uma matriz Ubuntu/macOS e Go 1.22/1.23. Ela executa `go vet`, testes com `-race`, build, verificações básicas da CLI e cross-compilation para Linux. Isso é evidência de sensores de desenvolvimento configurados; não é evidência de que o programa já tenha sido validado em toda distribuição Linux, ambiente corporativo ou fluxo real de usuário.

---

## O perfil separado ajuda, mas não é um sandbox

O comando `agyo browser start` inicia Chrome com um `--user-data-dir` próprio e uma porta de depuração local. Isso reduz a mistura acidental com o perfil pessoal do navegador e permite conectar ferramentas de automação ao Chrome gerenciado pela CLI.

Há duas ressalvas que não quero esconder. Primeiro, o endpoint CDP é uma interface privilegiada de controle do navegador: quem consegue acessá-lo pode inspecionar e conduzir aquela sessão. O perfil separado reduz o alcance aos dados do Chrome pessoal, mas não transforma páginas, extensões ou automações em código confiável.

Segundo, em ambientes Linux sem display, o `agyo` acrescenta flags de execução headless, incluindo `--no-sandbox`. Essa opção pode ser necessária em alguns ambientes restritos, mas desativa uma proteção do Chrome. Eu trataria esse modo como uma escolha de ambiente com risco conhecido, não como uma configuração universalmente segura.

Também vale olhar o que acontece com a memória no Git: os arquivos Markdown ficam dentro do projeto. O `.agents/.gitignore` inicial ignora logs e alguns diretórios temporários, mas não ignora `state.md`, `decisions.md` ou `todo.md`. Portanto, esses registros podem entrar em commits, conforme as regras do repositório. Não coloque segredos ou dados pessoais neles; decida explicitamente se a memória deve ser versionada.

---

## Por que Go e uma CLI pequena?

Escolhi Go para empacotar templates e rotinas operacionais num binário sem CGO. A distribuição não precisa de um runtime Python ou Node para executar `agyo`; o `npx` só é verificado porque os MCPs configurados podem depender dele.

A CLI também dá um ponto de entrada simples para tarefas que agentes ou pessoas podem chamar sem duplicar scripts de inicialização. Isso não elimina dependências do sistema: Chrome ou Chromium precisa estar instalado para automação de navegador, e o ambiente precisa fornecer os executáveis que os diagnósticos procuram.

O ponto de partida é pequeno: instalar, executar `agyo doctor`, inicializar a sessão no repositório de trabalho e só então escolher quais ferramentas usar. O código e as instruções estão no [repositório público](https://github.com/tiagovilasboas/antigravity-operator), junto com o [README em português](https://github.com/tiagovilasboas/antigravity-operator/blob/main/README.pt-BR.md).

---

## O que o projeto ainda não promete

O Antigravity Operator é um projeto comunitário; não é patrocinado nem mantido oficialmente pelo Google. Ele não cria um sandbox completo para comandos, não prova que um agente seguiu todas as regras e não garante privacidade só por manter estado local. O resultado depende das permissões do usuário, das instruções carregadas, das ferramentas conectadas e dos sensores disponíveis no repositório.

Essa fronteira é importante para mim. Um operador pode diminuir setup repetido e tornar o estado mais visível, mas a autonomia segura depende do conjunto: modelo, harness do fornecedor e user harness, incluindo revisão humana onde a consequência pede julgamento.

Minha hipótese é que uma CLI pequena pode transformar parte desse harness em operação reproduzível entre sessões e máquinas. O que ainda precisa ser medido é se isso reduz falhas de setup e retrabalho em ambientes de uso variados - não basta contar comandos ou linhas de teste.

Se você montou uma camada operacional para coding agents, qual parte mais te deu trabalho: memória entre sessões, permissões, navegador isolado ou diferenças entre máquinas?
