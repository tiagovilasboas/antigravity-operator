# Antigravity Operator (`agyo`)

<p align="center">
  <a href="https://github.com/tiagovilasboas/antigravity-operator/releases"><img src="https://img.shields.io/github/v/release/tiagovilasboas/antigravity-operator?style=flat-square&logo=github&color=blue" alt="Release" /></a>
  <a href="https://github.com/tiagovilasboas/antigravity-operator/actions"><img src="https://img.shields.io/github/actions/workflow/status/tiagovilasboas/antigravity-operator/ci.yml?style=flat-square&logo=githubactions&logoColor=white&label=CI" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/tiagovilasboas/antigravity-operator"><img src="https://goreportcard.com/badge/github.com/tiagovilasboas/antigravity-operator?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-000000?style=flat-square&logo=apple&logoColor=white" alt="Platform" />
  <img src="https://img.shields.io/badge/Architecture-Single%20Binary%20(No%20CGO)-success?style=flat-square" alt="Binary" />
  <img src="https://img.shields.io/badge/Coverage->80%25-brightgreen?style=flat-square" alt="Coverage" />
  <img src="https://img.shields.io/badge/Pattern-Fowler%20Outer%20Harness-blueviolet?style=flat-square" alt="Pattern" />
  <img src="https://img.shields.io/badge/Sponsor-GitHub%20Sponsors-ea4aaa?style=flat-square&logo=githubsponsors" alt="Sponsor" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

> **The Autonomous Session Agent Engine & OS Runtime**  
> *Transforme o Google Antigravity e outros clientes de IA em operadores autônomos de sistema operacional (estilo Claude Computer Use / OS Agent), com governança rígida, memória de sessão persistente no disco e paridade total entre macOS e Linux.*

```text
┌─── Antigravity Operator (agyo) ────────────────────────────────────────────────────────┐
│ $ agyo doctor                                                                          │
│ 🔍 Antigravity Operator Doctor [OS: darwin | Arch: arm64]                             │
│ 🖥️  Display Server: Detected (Desktop GUI)                                             │
│ -----------------------------------------------------------------                      │
│ ✅ Git                          : git version 2.39.5 (Tiago Vilas Boas)                │
│ ✅ Google Antigravity           : Ativo (5 processos detectados, PID primário: 71409)   │
│ ✅ Google Chrome                : Localizado em: /Applications/Google Chrome.app       │
│ ✅ Chrome DevTools (Port 9222)  : Ativo (Chrome/153.0) no perfil isolado               │
│ ✅ NPX (MCP Runtime)            : Versão 10.8.2 disponível                             │
│ ℹ️  Gemini API Key (BYOK)        : Configurada via GEMINI_API_KEY (AIza...9876)         │
│ ✅ Harness Core                 : Conectado em ~/Github/harness-core                   │
│                                                                                        │
│ $ agyo session watch --once --steps 2                                                  │
│ 📡 Streaming Antigravity Brain [db9011ea]                                              │
│ 💭 [Think #1242] Analyzing architecture trade-offs...                                  │
│ 🛠️  [Tool #1242] replace_file_content(watcher.go)                                       │
│ 🔔 [INTERAÇÃO #1243] O agente precisa da sua resposta! (Alerta visual + sonoro)        │
│                                                                                        │
│ $ agyo browser tabs                                                                    │
│ 🌐 Open Chrome Tabs (1 active):                                                        │
│ [7F13B00E] Google AI Developers Forum : https://discuss.ai.google.dev                 │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

> **Aviso:** *Este é um projeto comunitário de código aberto e não é um produto oficial patrocinado pelo Google. Foi concebido para estender e potencializar o ecossistema do Google Antigravity e desenvolvedores Google AI.*

---

## 📑 Sumário

- [Visão Geral](#-visão-geral)
- [Presente para a Comunidade & Google AI Pro](#-um-presente-de-engenharia-para-a-comunidade--para-o-google)
- [O Problema: Por Que o Antigravity Precisa de um Operator?](#-o-problema-por-que-o-antigravity-precisa-de-um-operator)
- [A Solução & Recursos Principais](#-a-solução-o-que-o-antigravity-operator-resolve)
- [Comparativo de Mercado](#-comparativo-onde-o-agyo-se-posiciona)
- [Caso de Uso para Estudantes e Google AI Pro](#-caso-de-uso-de-destaque-estudantes-pesquisa--google-ai-pro)
- [Arquitetura do Sistema (SRP, KISS, YAGNI, DRY)](#-arquitetura-canônica-srp-kiss-yagni-dry)
- [Instalação e Início Rápido](#-instalação-rápida)
- [Como Usar a CLI](#-como-usar)
- [Padrões de Comunidade e Contribuição](#-contribuindo-e-padrões-da-comunidade)
- [Segurança e Licença](#-segurança-e-licença)

---

### 🎁 Um Presente de Engenharia para a Comunidade & para o Google

> *"Este projeto é uma contribuição aberta da comunidade para todos os desenvolvedores, estudantes e pesquisadores, e um agradecimento especial ao **Google** pelo incentivo transformador concedido aos estudantes através do plano **Google AI Pro**."*  
>  
> Nosso objetivo é democratizar a engenharia de ponta: permitir que qualquer estudante ou desenvolvedor utilize 100% do poder do **Google Antigravity e dos modelos Gemini Pro** com rigor profissional, sem queimar cotas de API com loops desgovernados e com portabilidade absoluta para qualquer laboratório Linux ou macOS.

---

## 🔍 O Problema: Por Que o Antigravity Precisa de um Operator?

O Google Antigravity é uma das plataformas de desenvolvimento assistido por IA mais poderosas da atualidade — possui ferramentas atômicas nativas de shell, edição cirúrgica, MCPs e subagentes. 

**Porém, "de fábrica", o Antigravity é uma engine de força bruta sem um Harness de Sessão embutido:**
* **Concorrentes já empacotam guard-rails:** Ferramentas como Claude Code, Devin ou Cursor possuem convenções prontas ou sandboxes fechadas. O Antigravity te entrega as ferramentas atômicas puras (`run_command`, `write_to_file`), mas **não entrega a camada de governança e controle de sessão**.
* **Sem memória de longo prazo:** Ele não possui um protocolo nativo de persistência de estado no disco, gerando amnésia operacional a cada nova janela de contexto.
* **Sem automação de ambiente de runtime:** O usuário precisa configurar manualmente portas CDP, isolamento de perfis de browser e contornar peculiaridades de Linux na unha.

### As 4 Falhas Críticas de Agentes sem Governança:

1. **A Síndrome do "Agente Bêbado" no Terminal:** Agentes que ganham acesso a shell e executam comandos sem freio, alucinam diretórios, assumem que código funciona sem testar e entram em loops infinitos consumindo tokens.
2. **Amnésia Operacional e Estouro de Contexto:** Conforme a conversa avança, o agente esquece o objetivo principal, descarta decisões arquiteturais combinadas e repete erros já cometidos.
3. **Invasão e Risco no Navegador Pessoal:** Agentes que tentam interagir com a web sequestrando ou sujando o navegador pessoal do desenvolvedor, expondo cookies privados, abas de trabalho ou travando por falta de flags corretas.
4. **O Abismo Mac vs Linux:** Automações que funcionam no macOS com interface gráfica quebram miseravelmente quando levadas para um servidor Linux, VPS, WSL2 ou container Docker (ausência de servidor X11/Wayland, crashes de memória em `/dev/shm` e permissões de sandbox).

---

## 💡 A Solução: O Que o `antigravity-operator` Resolve

O **`antigravity-operator`** (`agyo`) empacota toda a infraestrutura operacional, segurança e governança para que o seu agente atue como um **engenheiro de software e operador de sistemas sênior**:

* **Outer Harness de Martin Fowler (Guia × Sensor):** O agente nunca assume nada sem evidência direta. Guias alimentam o agente antes da ação; sensores computacionais (`go test`, linters, verificação de runtime) validam a entrega antes de declarar a tarefa pronta.
* **Memória Operacional Persistente (`.agents/session/`):** Transições de estado vivem no filesystem do projeto (`state.md`, `decisions.md`, `todo.md`). O agente mantém coerência perfeita mesmo se a janela de chat reiniciar.
* **Isolamento Total do Chrome via DevTools MCP:** Lança uma instância dedicada do Chrome com porta de depuração (`9222`) e perfil isolado (`~/.gemini/antigravity-browser-profile`), garantindo zero poluição do seu navegador pessoal.
* **Adaptação Inteligente Headless (Linux & Servidores):** Detecta dinamicamente a presença de display gráfico (`$DISPLAY` / `$WAYLAND_DISPLAY`). Se não houver tela, ativa automaticamente `--headless=new`, `--disable-dev-shm-usage` e `--no-sandbox`.
* **Zero Runtime Dependencies (Single Binary Go):** Compilado em Go puro (`CGO_ENABLED=0`), gerando um executável estático único de ~6MB que você pode copiar para qualquer Linux ou Mac e rodar na hora, sem instalar Python, Node ou gerenciadores de pacotes.

---

## 🥊 Comparativo: Onde o `agyo` se Posiciona

| Recurso | Scripts Soltos / Bash | Claude Computer Use | Open-Interpreter | **Antigravity Operator (`agyo`)** |
|---|---|---|---|---|
| **Governança Outer Harness** | ❌ Não | ❌ Não | ❌ Não | **✅ Nativo (Guia × Sensor)** |
| **Memória Operacional em Disco** | ❌ Não | ❌ Não | ❌ Não | **✅ `.agents/session/` Canônico** |
| **Browser Profile Isolado** | ❌ Usa pessoal | ⚠️ Container pesado | ❌ Não | **✅ Perfil Dedicado Seguro** |
| **Paridade macOS / Linux** | ⚠️ Quebra fácil | ⚠️ Docker-only | ⚠️ Conflito de deps | **✅ Nativo & Headless Auto** |
| **Dependências de Instalação** | Múltiplas | Docker / APIs | Python / venv / pip | **✅ Binário Único Estático** |
| **Sensor de Ambiente (`doctor`)** | ❌ Não | ❌ Não | ❌ Não | **✅ Integrado na CLI** |

---

## 🎓 Caso de Uso de Destaque: Estudantes, Pesquisa & Google AI Pro

Para estudantes de tecnologia, computação e engenharia que utilizam os benefícios de planos acadêmicos como o **Google AI Pro**, o `antigravity-operator` se torna o multiplicador de aprendizado definitivo:

1. **Eficiência de Cota e Zero Desperdício de Tokens:** Agentes desgovernados consom cotas generosas de API em minutos devido a loops de erro e alucinação. Com os princípios de *Outer Harness*, o consumo de tokens é cirúrgico e focado no problema real.
2. **Ambiente Portátil para Laboratórios da Faculdade (Linux sem Root):** Computadores de universidades e centros de pesquisa rodam Linux onde o estudante não possui privilégios de administrador (`root`) para instalar Docker ou dependências globais. O binário estático `agyo-linux-amd64` roda direto da pasta do usuário (`~/`), sem necessitar de permissões especiais.
3. **Diário de Bordo de Estudos & Portfólio:** A pasta `.agents/session/` registra o histórico técnico, trade-offs de algoritmos e decisões de código, servindo como documentação viva do aprendizado.
4. **Laboratório Seguro:** Navegação via DevTools MCP com perfil isolado impede que o agente acesse contas pessoais, senhas ou dados da universidade.

### 🎁 Skills para Estudantes Incluídas de Brinde (`skills/`):
O repositório já inclui 3 skills prontas para acelerar a rotina acadêmica:
* **`feynman-code-tutor`:** Tutor sênior baseado na Técnica Feynman. Explica algoritmos, estruturas de dados e Big-O com analogias do mundo real e perguntas de fixação.
* **`student-study-planner`:** Decompõe ementas pesadas, projetos finais e matérias complexas em sprints gerenciáveis de estudo focado (20% teoria, 80% código).
* **`token-budget-guard`:** Guardião cirúrgico que impede respostas repetitivas ou leitura desnecessária de arquivos, estendendo a longevidade da sua cota do Google AI Pro.

---

## 🏛️ Arquitetura Canônica (SRP, KISS, YAGNI, DRY)

```text
antigravity-operator/
├── cmd/agyo/                 # Entrypoint da CLI (main.go)
├── internal/
│   ├── platform/             # SRP: Detecção de SO, display X11/Wayland e caminhos do Chrome
│   ├── session/              # SRP: Scaffold da memória operacional (.agents/session/) e proteção de logs
│   ├── profile/              # SRP: Gerenciamento do Chrome com perfil isolado e endpoint CDP 9222
│   ├── watcher/              # SRP: Streaming de raciocínio, árvore de subagentes e notificações de OS
│   ├── exporter/             # SRP: Exportador de relatórios consolidados de missão (Markdown e HTML)
│   ├── dashboard/            # SRP: Mini-servidor HTTP embutido em Go puro, UI web e API REST
│   ├── completion/           # SRP: Gerador de autocompletion de shell (Bash, Zsh, Fish)
│   ├── hook/                 # SRP: Sensor e guarda de continuidade de sessão para o Git pre-commit
│   ├── installer/            # SRP: Sincronização idempotente de regras e manifestos MCP
│   └── doctor/               # SRP: Sensor computacional de diagnóstico completo da máquina
├── templates/                # Embutido no binário estático via //go:embed (zero dependências)
│   ├── rules/                # Regras canônicas de Session Agent
│   ├── session/              # Templates de state.md, decisions.md e todo.md
│   └── mcps/                 # Manifesto de servidores MCP (DevTools, Playwright)
├── Formula/                  # Fórmula oficial do pacote Homebrew (agyo.rb)
├── agents/                   # Personas especializadas para engenharia assistida por IA
├── skills/                   # Habilidades bônus para estudantes (feynman tutor, study planner, token guard)
├── docs/                     # Especificações de arquitetura e artigos de lançamento (TabNews, LinkedIn)
├── .github/                  # CI/CD workflows, release automation e templates comunitários
├── scripts/                  # Instalador universal (install.sh) e setup de desenvolvimento (setup-dev.sh)
├── AUTHORS                   # Autores do projeto
├── CONTRIBUTORS              # Colaboradores do projeto
├── CODE_OF_CONDUCT.md        # Diretrizes comunitárias padrão Google Open Source
├── SECURITY.md               # Política de divulgação responsável de vulnerabilidades
├── PRIVACY.md                # Política de privacidade local-first e zero-telemetria (LGPD & GDPR)
├── CONTRIBUTING.md           # Guia de contribuição e protocolo de testes
└── Makefile                  # Alvos de build nativo, lint, coverage e cross-compilação
```

> 📖 **Especificação Detalhada de Engenharia e Arquitetura:**  
> Para uma análise aprofundada de cada subsistema, implementação RFC 6455 do CDP WebSocket em Go puro, algoritmos de memória $O(1)$ e decisões de design, leia o [Índice de Especificações do Sistema (docs/spec/)](docs/spec/README.md).

---

## 🤖 Assisted-IA & Ecossistema de Agentes

O projeto foi concebido sob o paradigma **Agent-as-Code** e traz governança de ponta:
- **`AGENTS.md`:** Contrato operacional de conduta, diretrizes de código Go, checklist de sensores computacionais e convenções de commit para qualquer IA (Antigravity, Cursor, Claude, Copilot).
- **Roster de Especialistas (`agents/`):**
  - **`operator-architect`:** Guardião do sistema operacional, paridade macOS/Linux e princípios KISS/YAGNI.
  - **`cdp-engineer`:** Especialista no Chrome DevTools Protocol, flags de browser e sockets de depuração.
  - **`qa-sentinel`:** Responsável pelos testes automatizados e sensores de regressão.

---

## ⚡ Instalação e Início Rápido

### Opção 1: Instalador Universal em Uma Linha (Recomendado / Zero Config)
Instala os binários estáticos diretamente no macOS ou Linux (sem necessidade de ter Go instalado):
```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash
```

### Opção 2: Homebrew (macOS & Linuxbrew)
```bash
brew install tiagovilasboas/tap/agyo
```

### Opção 3: Compilar do Código Fonte (Go 1.27+, ver go.mod)
```bash
git clone https://github.com/tiagovilasboas/antigravity-operator.git
cd antigravity-operator
make build
make install
```

---

## 🚀 Como Usar

### 1. Diagnosticar o ambiente da máquina (`doctor`)
Audita se o sistema operacional, Git, Chrome, Node/NPX e conexões de harness estão prontos:
```bash
agyo doctor
```bash
agyo doctor

# Saída em formato JSON para extensões e scripts:
agyo doctor --json

# Sensor de autorrecuperação (Self-Healing): repara arquivos ausentes ou corrompidos:
agyo doctor --fix
```

### 2. Inicializar a memória operacional no projeto atual (`init`)
Cria a pasta `.agents/session/` com rastreamento de estado, decisões e tarefas, além do `.agentignore`:
```bash
cd meu-projeto
agyo init
```

Estrutura criada:
- `.agents/session/state.md` (Objetivo e status em tempo real)
- `.agents/session/decisions.md` (Log de premissas e trade-offs arquiteturais)
- `.agents/session/todo.md` (Tarefas em andamento, pendentes e concluídas)
- `.agents/.gitignore` (Protegendo logs e credenciais contra commits acidentais)
- `.agentignore` (Lista negra de tokens: exclui `node_modules/`, `vendor/`, lockfiles e dumps)

### 3. Inspecionar e Gerenciar a Memória de Sessão (`session`)
Acompanhe o progresso ativo, faça streaming do raciocínio em tempo real ou arquive missões concluídas:
```bash
# Ver objetivo atual, fase e porcentagem de tarefas concluídas (suporta --json):
agyo session status
agyo session status --json

# Compactar tarefas concluídas para evitar context bloat de tokens:
agyo session compact
agyo session compact --dry-run

# Acompanhar raciocínio e chamadas de ferramentas em tempo real com notificações nativas:
agyo session watch

# Inspecionar a árvore de subagentes ativos e mensagens inter-agentes:
agyo session watch --once --steps 10 --tree

# Exportar relatório consolidado da sessão em Markdown ou HTML responsivo completo:
agyo session export --format=markdown
agyo session export --format=html --out=relatorio-sessao.html

# Arquivar sessão concluída para o histórico e resetar templates para a próxima tarefa:
agyo session archive

# Listar histórico de sessões arquivadas (suporta --json):
agyo session list
agyo session list --json

# Restaurar sessão arquivada para a memória ativa (gera backup preventivo automático):
agyo session restore session-2026-10-01-113140.md
agyo session restore latest
```

### 4. Checkpoints Atômicos & Botão de Pânico (`checkpoint` & `rollback`)
Proteja seu repositório contra refatorações alucinadas ou destrutivas de agentes de IA:
```bash
# Criar snapshot atômico antes de um refactor complexo:
agyo checkpoint "pre-refactor" --desc="Antes de alterar migrations do banco"

# Listar checkpoints salvos (suporta --json):
agyo checkpoint --list
agyo checkpoint --list --json

# Botão de pânico: reverte alterações locais e restaura a working tree exata:
agyo rollback
agyo rollback chk-20261001-113000
```

### 5. Dashboard Web & Inspetor Local em Tempo Real (`dashboard`)
Inicia um servidor web em Go puro com zero dependências externas em `http://127.0.0.1:8080`, com UI dark mode moderna, progresso de sessão, diagnósticos do host, abas ativas do Chrome e live activity log:
```bash
# Iniciar dashboard e abrir automaticamente no navegador padrão:
agyo dashboard

# Iniciar em porta customizada sem abrir o navegador automaticamente:
agyo dashboard --port 8090 --open=false
```

### 5. Gerenciar o Chrome isolado e Inspeção CDP (`browser`)
Controla o ciclo de vida da instância exclusiva do Chrome e permite inspeção direta via Chrome DevTools Protocol em Go puro (sem Node/Python):
```bash
# Iniciar normalmente (abre janela no Mac/Linux desktop na porta 9222):
agyo browser start

# Iniciar em porta customizada ou forçar modo headless (automático em servidores Linux/VPS sem tela):
agyo browser start --port 9223
agyo browser start --headless

# Verificar se a porta de debug e o PID estão ativos:
agyo browser status

# Inspeção nativa via Chrome DevTools Protocol (CDP em Go puro):
agyo browser tabs                    # Lista abas abertas e target IDs
agyo browser open https://github.com # Abre URL em nova aba
agyo browser close <targetId>        # Fecha aba por target ID
agyo browser eval "document.title"   # Executa JS na aba ativa
agyo browser shot screenshot.png     # Captura screenshot PNG da aba ativa

# Encerrar graciosamente o processo do Chrome isolado (SIGTERM):
agyo browser stop
```

### 6. Git Pre-Commit Hook de Continuidade (`hook`)
Instala um sensor automático em `.git/hooks/pre-commit` para evitar commits sem atualizar o objetivo e as tarefas concluídas da sessão:
```bash
# Instalar o hook no repositório atual (ou diretório especificado):
agyo hook install

# Desinstalar o hook quando necessário:
agyo hook uninstall
```

### 7. Sincronizar regras, skills e MCPs no Antigravity (`sync`)
Garante que as regras de governança e servidores de automação estejam instalados:
```bash
agyo sync
```
O `sync` nunca sobrescreve um manifesto MCP existente (`~/.gemini/antigravity/mcp/default-servers.json`). O `agyo doctor` avisa quando esse arquivo tem pacotes `npx` sem versão fixa ou difere do template embutido. Para fazer backup (`default-servers.json.bak-<timestamp UTC>`) e reescrevê-lo:
```bash
agyo sync --update-mcp
```

### 8. Autocompletar no Terminal (`completion`)
Gera scripts de autocompletion de comandos e flags para Zsh, Bash ou Fish:
```bash
# Zsh (adicione ao seu ~/.zshrc):
source <(agyo completion zsh)

# Bash (adicione ao seu ~/.bashrc):
source <(agyo completion bash)

# Fish:
agyo completion fish | source
```

### 9. Sobre o projeto e manifesto (`about`)
```bash
agyo about
```

---

## 🛡️ Princípios Operacionais Canônicos

Quando o Antigravity opera sob o `agyo`, ele segue 5 mandamentos:
1. **Autonomia de Investigação:** Busca fatos no terminal, browser e logs antes de fazer perguntas triviais.
2. **Orquestração Multiferramenta:** Identifica -> Investiga -> Implementa -> Testa -> Valida no Browser.
3. **Validação Rigorosa:** A tarefa só termina quando o resultado foi validado de ponta a ponta com evidências.
4. **Perfil Isolado:** Zero interferência ou exposição no Chrome pessoal do usuário.
5. **Comunicação Concisa:** Direta ao ponto, técnica e fundamentada em dados.

---

## 🤝 Como Contribuir

Contribuições de engenheiros, estudantes e entusiastas de open source são muito bem-vindas! Seja corrigindo bugs, aprimorando a documentação ou criando novas skills.

### Fluxo de Contribuição em 5 Passos:
1. **Fork & Clone:**
   ```bash
   git clone https://github.com/tiagovilasboas/antigravity-operator.git
   cd antigravity-operator
   ```
2. **Setup Automatizado de Dev (configura hooks de pre-commit):**
   ```bash
   ./scripts/setup-dev.sh
   ```
3. **Crie uma Branch de Funcionalidade:**
   ```bash
   git checkout -b feat/sua-funcionalidade
   ```
4. **Execute os Sensores e Testes:**
   ```bash
   go vet ./...
   go test -v -race ./...
   make build
   ./bin/agyo doctor
   ```
5. **Abra um Pull Request:** Siga o padrão [Conventional Commits](https://www.conventionalcommits.org/) em inglês e submeta seu PR.

### 💡 Ideias de Contribuição de Alto Impacto:
* **Skills para Estudantes:** Crie novas skills acadêmicas na pasta `skills/` (ex: tutor de grafos, revisor de monografia/TCC).
* **Integrações de MCP:** Adicione templates de servidores MCP úteis em `templates/mcps/`.
* **Testes em Outras Distribuições Linux:** Documente a compatibilidade no Arch Linux, Alpine, Fedora ou NixOS.

Consulte também [CONTRIBUTING.md](CONTRIBUTING.md) (o que é aceito), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [AUTHORS](AUTHORS) e [CONTRIBUTORS](CONTRIBUTORS).

---

## 💖 Sponsor & Apoie o Projeto

Este projeto é uma iniciativa independente e de código aberto criada para empoderar estudantes e desenvolvedores que utilizam o ecossistema Google AI e Google Antigravity.

Se o `antigravity-operator` economizou seu tempo, protegeu sua cota de tokens ou ajudou em seus estudos:

* ⭐ **Deixe uma Estrela (Star) no Repositório:** É a forma mais fácil e direta de ajudar o projeto a alcançar mais pessoas e chamar a atenção do time do Google.
* 💖 **Patrocine no GitHub Sponsors:** Ajude a custear os servidores de integração contínua e testes multi-OS através do [GitHub Sponsors](https://github.com/sponsors/tiagovilasboas).
* 🗣️ **Compartilhe com sua Comunidade:** Fale sobre o projeto no LinkedIn, X/Twitter, Discord ou grupos de estudo da faculdade.

---

## 🔒 Segurança, Privacidade & Licença

* **Política de Segurança:** Consulte [SECURITY.md](SECURITY.md) para diretrizes de divulgação responsável de vulnerabilidades.
* **Privacidade e Conformidade LGPD/GDPR:** Consulte [PRIVACY.md](PRIVACY.md) para detalhes sobre a política local-first, zero-telemetria e soberania total de dados sob a LGPD (Lei Federal 13.709/2018).
* **Licença:** Distribuído sob a licença [MIT](LICENSE).

