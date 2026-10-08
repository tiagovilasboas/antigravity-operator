# Antigravity Operator (`agyo`)

<p align="center">
  <a href="https://github.com/tiagovilasboas/antigravity-operator/releases"><img src="https://img.shields.io/github/v/release/tiagovilasboas/antigravity-operator?style=flat-square&logo=github&color=blue" alt="Release" /></a>
  <a href="https://github.com/tiagovilasboas/antigravity-operator/actions"><img src="https://img.shields.io/github/actions/workflow/status/tiagovilasboas/antigravity-operator/ci.yml?style=flat-square&logo=githubactions&logoColor=white&label=CI" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/tiagovilasboas/antigravity-operator"><img src="https://goreportcard.com/badge/github.com/tiagovilasboas/antigravity-operator?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Versão Go" />
  <img src="https://img.shields.io/badge/Plataforma-macOS%20%7C%20Linux%20%7C%20Windows-000000?style=flat-square&logo=apple&logoColor=white" alt="Plataforma" />
  <img src="https://img.shields.io/badge/Arquitetura-Bin%C3%A1rio%20%C3%9Anico%20(Sem%20CGO)-success?style=flat-square" alt="Binário" />
  <img src="https://img.shields.io/badge/Padr%C3%A3o-Fowler%20Outer%20Harness-blueviolet?style=flat-square" alt="Padrão" />
  <img src="https://img.shields.io/badge/Licen%C3%A7a-MIT-blue?style=flat-square" alt="Licença" />
</p>

> **O runtime de sessão e outer harness em binário único para o Google Antigravity.**  
> Conecta o **Google NotebookLM** (Cérebro de Pesquisa) ao **Antigravity** (Mãos de Execução) para transformar playlists do YouTube, artigos científicos e livros técnicos em artigos aprofundados, roteiros de estudo e código verificado com **~99% de economia de tokens** e zero poluição de contexto.

<p align="center">
  <a href="#início-rápido"><b>Início Rápido</b></a> •
  <a href="#por-que-o-agyo"><b>Por Que o agyo?</b></a> •
  <a href="#superpoderes--recursos"><b>Superpoderes</b></a> •
  <a href="#referência-da-cli"><b>Referência da CLI</b></a> •
  <a href="#arquitetura"><b>Arquitetura</b></a> •
  <a href="docs/spec/README.md"><b>Especificações</b></a> •
  <a href="README.md">🇺🇸 <b>English</b></a>
</p>

---

## Início Rápido

Instale o binário estático sem dependências de runtime:

```bash
# macOS & Linux (Instalador universal)
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash

# Ou via Homebrew
brew install tiagovilasboas/tap/agyo
```

Dentro de qualquer repositório git:

```bash
agyo init             # Cria a memória em .agents/session/ e o .agentignore
agyo session status   # Exibe objetivo atual, fase e progresso das tarefas
agyo dashboard        # Abre o dashboard local em http://127.0.0.1:8080
```

![agyo dashboard](docs/assets/dashboard.png)

> **Aviso:** *O `agyo` é um projeto comunitário independente e não é um produto oficial do Google. Ele foi criado para estender o ecossistema do Google Antigravity e Google AI.*

---

## Por Que o agyo?

Agentes de código são excelentes na execução atômica de tarefas, mas sofrem de dois problemas críticos: **amnésia operacional** (perda de contexto arquitetural entre janelas de contexto) e **desperdício de tokens** (colocar documentações densas ou transcrições de vídeo diretamente no prompt, esgotando cotas rapidamente).

O `agyo` implementa o modelo canônico de **Outer Harness (Guia × Sensor)** de Martin Fowler. Ele combina o **Google NotebookLM como Cérebro de Pesquisa fundamentado** e o **Antigravity como Mãos de Execução autônomas**.

### O Fluxo: Cérebro & Mãos

```text
[ Vídeo do YouTube / Aula / Artigo ]
                │
                ▼
      [ Google NotebookLM ] ── Índice multimodal, transcrições e citações com timestamp a custo 0 de tokens
                │
                ▼ (Pure-Go CDP / RFC 6455 WebSocket / stdio MCP)
      [ agyo notebooklm / MCP ] ── Busca fundamentada via `agyo notebooklm ask` ou MCP
                │
                ▼
      [ Google Antigravity ] ── Sintetiza artigos técnicos estruturados, spikes e código validado
                │
                ▼
[ ~/Estudos/artigos/*.md + Testes ] ── Acervo perene de estudos, arquitetura documentada e código testado
```

### Economia de Tokens (FinOps)

| Estratégia | Custo em Sessão de 15 Turnos | Limites de Taxa & Memória |
|---|---|---|
| **Transcrições brutas no prompt** (vídeo de 1h) | **~1.500.000 tokens** | Esgotamento rápido de TPM; forte amnésia (*Lost in the Middle*) |
| **`agyo` + NotebookLM (RAG Fundamentado)** | **~12.000 tokens** | **~99,2% de economia de cota**; citações precisas com timestamp |

---

## Superpoderes & Recursos

| Superpoder | O que faz na prática | Especificação |
|---|---|---|
| **Cérebro & Mãos (NotebookLM)** | Externaliza conhecimento denso via CDP nativo e servidor stdio MCP. | [Spec 08](docs/spec/08-NOTEBOOKLM-E-MCP-SERVER.md) |
| **Memória Determinística de Sessão** | Mantém `.agents/session/` (`state.md`, `decisions.md`, `todo.md`) persistente. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| **Botão de Pânico & Checkpoints** | Snapshots atômicos no git para reverter edições destrutivas instantaneamente. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| **Chrome Supervisionado e Isolado** | Perfil dedicado na porta `9222` (`~/.gemini/antigravity-browser-profile`) com fallback headless. | [Spec 05](docs/spec/05-SUPERVISAO-CHROME-E-CDP-PURO.md) |
| **Sensores & Auto-Cura** | Diagnósticos com `agyo doctor` e hooks de pre-commit para continuidade. | [Spec 07](docs/spec/07-SENSORES-DOCTOR-E-SELF-HEALING.md) |
| **Proteção de Tokens** | Cria `.agentignore` excluindo `node_modules/`, `vendor/`, dumps e lockfiles. | [Spec 03](docs/spec/03-TOKEN-GUARD-E-AGENTIGNORE.md) |

---

## Referência da CLI

| Comando | Descrição | Especificação |
|---|---|---|
| `agyo init` | Inicializa a estrutura `.agents/session/` e o arquivo `.agentignore`. | [Spec 01](docs/spec/01-ENTRADA-E-BOOTSTRAP.md) |
| `agyo session status` | Exibe objetivo ativo, fase atual e progresso das tarefas (suporta `--json`). | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo session watch` | Transmite o raciocínio do agente e ferramentas em tempo real com alertas do SO. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo session compact` | Compacta tarefas concluídas para manter o contexto do agente enxuto. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo session export` | Exporta relatório consolidado da sessão em Markdown ou HTML responsivo. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo checkpoint [nome]` | Cria um snapshot atômico da árvore de trabalho antes de refatores arriscados. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| `agyo rollback` | Reverte arquivos rastreados para o commit do último checkpoint. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| `agyo dashboard` | Inicia o painel visual local sem dependências em `http://127.0.0.1:8080`. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo browser start\|stop\|tabs` | Inicia e supervisiona o Chrome isolado usando protocolo CDP puro em Go. | [Spec 05](docs/spec/05-SUPERVISAO-CHROME-E-CDP-PURO.md) |
| `agyo notebooklm ask\|mcp` | Consulta o NotebookLM ou inicia o servidor stdio MCP para IDEs com agentes. | [Spec 08](docs/spec/08-NOTEBOOKLM-E-MCP-SERVER.md) |
| `agyo doctor [--fix]` | Diagnostica dependências do host, variáveis de display e corrige arquivos. | [Spec 07](docs/spec/07-SENSORES-DOCTOR-E-SELF-HEALING.md) |
| `agyo hook install` | Instala hook de git pre-commit para compactar tarefas e garantir continuidade. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo sync [--update-mcp]` | Sincroniza regras canônicas, skills e manifesto de MCPs no Antigravity. | [Spec 01](docs/spec/01-ENTRADA-E-BOOTSTRAP.md) |
| `agyo completion <shell>` | Gera scripts de autocompletar para Bash, Zsh ou Fish. | - |

> Especificação detalhada de cada comando e opções: [docs/spec/README.md](docs/spec/README.md).

---

## Arquitetura

Construído em Go puro (`CGO_ENABLED=0`) sem nenhuma dependência externa de runtime. Modelos e regras são embutidos diretamente no executável (`//go:embed`).

```text
antigravity-operator/
├── cmd/agyo/                 # CLI entrypoint (main.go)
├── internal/
│   ├── platform/             # SRP: OS detection, X11/Wayland check, Chrome binary resolution
│   ├── session/              # SRP: .agents/session/ scaffold & gitignore protection
│   ├── checkpoint/           # SRP: Atomic working-tree snapshots & rollback
│   ├── profile/              # SRP: Chrome lifecycle management, PID tracking & CDP port
│   ├── watcher/              # SRP: Brain streaming, subagent tree hierarchy & OS notifications
│   ├── exporter/             # SRP: Consolidated session report generator (Markdown & HTML)
│   ├── analytics/            # SRP: Transcript parsing, tool call metrics aggregation & secret redaction
│   ├── dashboard/            # SRP: Pure-Go embedded HTTP server, web UI & REST API
│   ├── completion/           # SRP: Shell autocompletion generator (Bash, Zsh, Fish)
│   ├── hook/                 # SRP: Git pre-commit continuity sensor & safeguards
│   ├── installer/            # SRP: Idempotent rule and MCP manifesto synchronization
│   ├── notebook/             # SRP: Native Google NotebookLM integration & stdio MCP server
│   └── doctor/               # SRP: Machine diagnostic computational sensors
├── templates/                # Embedded static assets via //go:embed (zero external deps)
├── skills/                   # Bonus student skills (video-to-article, feynman, etc.)
└── docs/                     # Especificações técnicas (docs/spec/) e guias de arquitetura
```

---

## Edição para Estudantes & Pesquisa

Uma homenagem aos estudantes de computação, pesquisadores e ao programa **Google AI Pro**:

* **Portabilidade Sem Root:** O executável estático roda direto no espaço de usuário (`~/`) em laboratórios Linux universitários, sem precisar de Docker ou `sudo`.
* **Skills Incluídas de Fábrica (`skills/`):**
  * `video-to-article`: Transforma vídeos e palestras do YouTube em artigos de estudo e POCs via NotebookLM.
  * `feynman-code-tutor`: Explica algoritmos complexos e Big-O com analogias práticas do cotidiano.
  * `student-study-planner`: Divide ementas e matérias de faculdade em ciclos focados 20/80.
  * `token-budget-guard`: Otimiza o consumo de tokens e previne loops repetitivos de contexto.

---

## Contribuição & Licença

Contribuições da comunidade são muito bem-vindas! Para compilar e rodar os testes localmente:

```bash
git clone https://github.com/tiagovilasboas/antigravity-operator.git
cd antigravity-operator
make test             # Executa go test -v -race ./...
make build            # Compila o binário estático em bin/agyo
```

Leia [CONTRIBUTING.md](CONTRIBUTING.md) e [SECURITY.md](SECURITY.md). Distribuído sob a [Licença MIT](LICENSE).
