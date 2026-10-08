# Antigravity Operator (`agyo`)

<p align="center">
  <a href="https://github.com/tiagovilasboas/antigravity-operator/releases"><img src="https://img.shields.io/github/v/release/tiagovilasboas/antigravity-operator?style=flat-square&logo=github&color=blue" alt="Release" /></a>
  <a href="https://github.com/tiagovilasboas/antigravity-operator/actions"><img src="https://img.shields.io/github/actions/workflow/status/tiagovilasboas/antigravity-operator/ci.yml?style=flat-square&logo=githubactions&logoColor=white&label=CI" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/tiagovilasboas/antigravity-operator"><img src="https://goreportcard.com/badge/github.com/tiagovilasboas/antigravity-operator?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-000000?style=flat-square&logo=apple&logoColor=white" alt="Platform" />
  <img src="https://img.shields.io/badge/Architecture-Single%20Binary%20(No%20CGO)-success?style=flat-square" alt="Binary" />
  <img src="https://img.shields.io/badge/Pattern-Fowler%20Outer%20Harness-blueviolet?style=flat-square" alt="Pattern" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

> **The single-binary session runtime and outer harness for Google Antigravity.**  
> Bridges **Google NotebookLM** (Research Brain) with **Antigravity** (Execution Hands) to turn YouTube playlists, academic papers, and technical books into deep articles, study guides, and verified code with **~99% token savings** and zero context bloat.

<p align="center">
  <a href="#quickstart"><b>Quickstart</b></a> •
  <a href="#why-agyo"><b>Why agyo?</b></a> •
  <a href="#core-capabilities"><b>Superpowers</b></a> •
  <a href="#cli-reference"><b>CLI Reference</b></a> •
  <a href="#architecture"><b>Architecture</b></a> •
  <a href="docs/spec/README.md"><b>Specs</b></a> •
  <a href="README.pt-BR.md">🇧🇷 <b>Português</b></a>
</p>

---

## Quickstart

Install the single static binary with no runtime dependencies:

```bash
# macOS & Linux (Universal installer)
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash

# Or via Homebrew
brew install tiagovilasboas/tap/agyo
```

Inside any git repository:

```bash
agyo init             # Scaffolds .agents/session/ memory and .agentignore
agyo session status   # Inspects current objective, phase, and tasks
agyo dashboard        # Launches local UI on http://127.0.0.1:8080
```

![agyo dashboard](docs/assets/dashboard.png)

> **Disclaimer:** *`agyo` is an independent, community-driven companion project and is not an officially sponsored Google product. It extends the Google Antigravity & Google AI developer ecosystem.*

---

## Why agyo?

AI coding agents excel at atomic execution but suffer from two critical flaws: **operational amnesia** (losing architectural context across turns) and **context bloat** (stuffing dense documentation or video transcripts directly into prompt tokens, quickly exhausting rate limits).

`agyo` implements Martin Fowler's canonical **Outer Harness (Guide × Sensor)** model. It pairs **Google NotebookLM as the grounded Research Brain** with **Antigravity as the autonomous Execution Hands**.

### The Workflow: Brain & Hands

```text
[ YouTube Video / Course / Paper ]
                │
                ▼
      [ Google NotebookLM ] ── Multimodal index, transcripts & timestamped citations at 0 token cost
                │
                ▼ (Pure-Go CDP / RFC 6455 WebSocket / stdio MCP)
      [ agyo notebooklm / MCP ] ── Grounded search via `agyo notebooklm ask` or MCP
                │
                ▼
      [ Google Antigravity ] ── Synthesizes structured articles, spikes, and verified implementations
                │
                ▼
[ ~/Estudos/artigos/*.md + Tests ] ── Perennial study assets, documented architectures, tested code
```

### Token Economics (FinOps)

| Strategy | 15-Turn Session Cost | Rate Limits & Memory |
|---|---|---|
| **Raw prompt stuffing** (1h video transcript) | **~1,500,000 tokens** | Frequent TPM throttling; severe "Lost in the Middle" amnesia |
| **`agyo` + NotebookLM Grounded RAG** | **~12,000 tokens** | **~99.2% token savings**; precise timestamped citations |

---

## Core Capabilities

| Capability | What it does | Spec |
|---|---|---|
| **Brain & Hands (NotebookLM)** | Externalizes dense multimodal knowledge via pure-Go CDP and stdio MCP server. | [Spec 08](docs/spec/08-NOTEBOOKLM-E-MCP-SERVER.md) |
| **Deterministic Session Memory** | Persists `.agents/session/` (`state.md`, `decisions.md`, `todo.md`) across chat resets. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| **Panic Button & Checkpoints** | Atomic git snapshots and rollback to instantly undo destructive agent edits. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| **Supervised Chrome Profile** | Dedicated browser on port `9222` (`~/.gemini/antigravity-browser-profile`) with auto-headless Linux. | [Spec 05](docs/spec/05-SUPERVISAO-CHROME-E-CDP-PURO.md) |
| **Sensors & Self-Healing** | `agyo doctor` host diagnostic probes and pre-commit continuity hooks. | [Spec 07](docs/spec/07-SENSORES-DOCTOR-E-SELF-HEALING.md) |
| **Token Waste Protection** | Scaffolds `.agentignore` to exclude `node_modules/`, `vendor/`, dumps, and lockfiles. | [Spec 03](docs/spec/03-TOKEN-GUARD-E-AGENTIGNORE.md) |

---

## CLI Reference

| Command | Description | Spec |
|---|---|---|
| `agyo init` | Scaffolds `.agents/session/` memory structure and `.agentignore`. | [Spec 01](docs/spec/01-ENTRADA-E-BOOTSTRAP.md) |
| `agyo session status` | Displays active objective, phase, and task progress (supports `--json`). | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo session watch` | Streams agent reasoning and tool executions live with desktop notifications. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo session compact` | Compacts completed tasks into an archive rollup to keep context lean. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo session export` | Generates consolidated session reports in Markdown or standalone HTML. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo checkpoint [name]` | Creates an atomic working-tree snapshot before complex refactoring. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| `agyo rollback` | Reverts tracked files to the latest checkpoint commit. | [Spec 04](docs/spec/04-FILESYSTEM-SAFETY-NET-CHECKPOINT.md) |
| `agyo dashboard` | Spawns zero-dependency local web inspector on `http://127.0.0.1:8080`. | [Spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md) |
| `agyo browser start\|stop\|tabs` | Launches and supervises isolated Chrome with pure-Go CDP protocol. | [Spec 05](docs/spec/05-SUPERVISAO-CHROME-E-CDP-PURO.md) |
| `agyo notebooklm ask\|mcp` | Queries Google NotebookLM or exposes stdio MCP server for agent IDEs. | [Spec 08](docs/spec/08-NOTEBOOKLM-E-MCP-SERVER.md) |
| `agyo doctor [--fix]` | Diagnoses host dependencies, display env, and repairs session files. | [Spec 07](docs/spec/07-SENSORES-DOCTOR-E-SELF-HEALING.md) |
| `agyo hook install` | Installs git pre-commit hook that compacts sessions and checks continuity. | [Spec 02](docs/spec/02-MEMORIA-OPERACIONAL-E-SESSAO.md) |
| `agyo sync [--update-mcp]` | Synchronizes canonical agent rules, skills, and MCP manifests. | [Spec 01](docs/spec/01-ENTRADA-E-BOOTSTRAP.md) |
| `agyo completion <shell>` | Outputs shell completion scripts for Bash, Zsh, or Fish. | - |

> Full command specs and options: [docs/spec/README.md](docs/spec/README.md).

---

## Architecture

Built in pure Go (`CGO_ENABLED=0`) with zero external runtime dependencies. Templates and rules are embedded directly into the binary (`//go:embed`).

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
└── docs/                     # Specifications (docs/spec/) & architecture guides
```

---

## Student & Research Edition

A tribute to computer science students, researchers, and the **Google AI Pro** access program:

* **Zero-Root Portability:** Static binaries run from user space (`~/`) in locked-down university Linux labs without Docker or `sudo`.
* **Bonus Skills Included (`skills/`):**
  * `video-to-article`: Transforms YouTube videos and lectures into study guides and runnable POCs via NotebookLM.
  * `feynman-code-tutor`: Explains complex algorithms and Big-O through real-world analogies.
  * `student-study-planner`: Breaks course syllabi into focused 20/80 sprint cycles.
  * `token-budget-guard`: Prevents wasteful context consumption.

---

## Contributing & License

We welcome contributions! To build and test locally:

```bash
git clone https://github.com/tiagovilasboas/antigravity-operator.git
cd antigravity-operator
make test             # Runs go test -v -race ./...
make build            # Compiles static binary to bin/agyo
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Distributed under the [MIT License](LICENSE).
