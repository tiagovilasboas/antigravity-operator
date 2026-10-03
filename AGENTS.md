# AGENTS.md — Contrato Operacional de IA para o antigravity-operator

Bem-vindo ao repositório **`antigravity-operator`** (`agyo`). Este documento define a governança técnica, os princípios canônicos e as regras operacionais que **qualquer agente autônomo ou assistido de IA** deve seguir obrigatoriamente neste projeto.

---

## 🏛️ Propósito do Repositório
O `antigravity-operator` é um motor de execução e gerenciamento de **Session Agents** autônomos. Ele transforma o Google Antigravity e outros clientes de IA em operadores de sistema operacional completos, seguros e com memória persistente de sessão, operando com paridade total entre **macOS e Linux**.

---

## 🧭 Princípios Canônicos (Fowler Outer Harness)

1. **Outer Harness (Guia × Sensor):**
   * **Guias:** Este documento, regras de arquitetura e steerings. Leia-os antes de gerar código.
   * **Sensores:** Todo código deve ser validado computacionalmente (`go test -v ./...`, `go vet ./...`, `agyo doctor`) antes de declarar a tarefa concluída.
2. **Sem Afirmação Sem Fonte (No Assumptions):**
   * Nunca assuma o estado do sistema ou caminhos de arquivos. Inspecione com ferramentas ou comandos reais.
3. **Mínima Alteração Necessária (Minimal Change Principle / SRP / KISS / YAGNI):**
   * Não adicione frameworks pesados ou abstrações prematuras.
   * Mantenha o design em pacotes pequenos e independentes dentro de `internal/`.
4. **Zero Runtime Dependencies (Single Binary):**
   * Todo binário gerado deve compilar com `CGO_ENABLED=0` estático.
   * Não adicione dependências de CGO ou bibliotecas dinâmicas externas.
5. **Templates Embutidos:**
   * Qualquer novo template de regra, manifesto MCP ou markdown de sessão deve residir em `templates/` e ser referenciado via `//go:embed` em `templates/templates.go`.

---

## 📂 Mapa de Arquitetura e Responsabilidades

| Pacote / Arquivo | Responsabilidade Canônica |
|---|---|
| `cmd/agyo/main.go` | Entrypoint da CLI, roteamento de subcomandos e saída ao usuário |
| `internal/platform/` | Detecção de SO, arquitetura, display gráfico (X11/Wayland) e caminhos do Chrome |
| `internal/session/` | Criação e garantia de integridade da pasta `.agents/session/` e `.agents/.gitignore` |
| `internal/profile/` | Gerenciamento do Chrome com perfil isolado e comunicação via CDP (porta 9222) |
| `internal/installer/` | Instalação idempotente de regras canônicas e manifestos MCP no host |
| `internal/doctor/` | Sensores de diagnóstico (Git, Chrome, NPX, Conexão com Harness Core) |
| `templates/` | Fonte estática de templates embutidos via `embed.FS` |

---

## 🧪 Sensores de Verificação (Checklist Obrigatório)

Antes de finalizar qualquer alteração ou propor commits, o agente DEVE executar:

```bash
# 1. Análise estática do Go
go vet ./...

# 2. Bateria completa de testes unitários
go test -v ./...

# 3. Compilação do binário local
make build

# 4. Teste de cross-compilação para Linux
make build-linux

# 5. Execução do sensor de integridade da própria ferramenta
./bin/agyo doctor
```

---

## 📜 Convenções de Git e Mensagens de Commit
- Todos os commits devem seguir a convenção **Conventional Commits** e ser escritos **estritamente em inglês**.
- Exemplos:
  - `feat(browser): add support for custom debug port flag`
  - `fix(platform): properly detect Wayland socket on Ubuntu 24.04`
  - `docs: update cross-platform compatibility matrix`

---

## 🤝 Contribution rules for AI agents

Same rules as [CONTRIBUTING.md](CONTRIBUTING.md#-what-gets-accepted); they apply to any agent working here.

- **Scope:** supervision, observability and session persistence for Antigravity. Pick work tied to a real operator pain.
- **Out of scope:** unredacted prompts, transcript content or commands in the API/UI; reading undocumented internal logs without an approved proposal; new dependencies (`go.mod` is stdlib only).
- **Dashboard:** stays the simple embedded single-file UI (spec 06). No frameworks, build steps or CDN assets.
- **Privacy by default** ([PRIVACY.md](PRIVACY.md)): local only, dashboard on localhost, redact before showing anything from a session.
- **One subject per PR**, ~400 lines of code (excluding tests/generated). Slice anything bigger; never bundle features.
- **Issue first** for a new feature or any UI/API change; agree on scope before coding.
- **Sensors before done:** `gofmt -l .` empty, `go vet ./...`, `go test -race ./...`, with tests for new behavior.
- **Keep only what delivers clear operator value.** Drop nice-to-have, cosmetic or unrequested changes; don't bundle or self-expand scope. When in doubt, ask the maintainer in an issue first.
- **The human author reviews and owns every line** an agent produces.
