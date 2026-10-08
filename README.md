# Antigravity Operator (`agyo`)

<p align="center">
  <a href="https://github.com/tiagovilasboas/antigravity-operator/releases"><img src="https://img.shields.io/github/v/release/tiagovilasboas/antigravity-operator?style=flat-square&logo=github&color=blue" alt="Release" /></a>
  <a href="https://github.com/tiagovilasboas/antigravity-operator/actions"><img src="https://img.shields.io/github/actions/workflow/status/tiagovilasboas/antigravity-operator/ci.yml?style=flat-square&logo=githubactions&logoColor=white&label=CI" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/tiagovilasboas/antigravity-operator"><img src="https://goreportcard.com/badge/github.com/tiagovilasboas/antigravity-operator?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-000000?style=flat-square&logo=apple&logoColor=white" alt="Platform" />
  <img src="https://img.shields.io/badge/Architecture-Single%20Binary%20(No%20CGO)-success?style=flat-square" alt="Binary" />
  <img src="https://img.shields.io/badge/Coverage->80%25-brightgreen?style=flat-square" alt="Coverage" />
  <img src="https://img.shields.io/badge/Pattern-Fowler%20Outer%20Harness-blueviolet?style=flat-square" alt="Pattern" />
  <img src="https://img.shields.io/badge/Sponsor-GitHub%20Sponsors-ea4aaa?style=flat-square&logo=githubsponsors" alt="Sponsor" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

> **The Autonomous Session Agent Engine & OS Runtime for Google Antigravity**  
> *Deterministic governance, filesystem operational memory, and Chrome DevTools isolation with seamless parity across macOS and Linux.*

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

<p align="center">
  <a href="README.pt-BR.md">🇧🇷 <b>Leia em Português</b></a> | <a href="#-getting-started--installation"><b>Getting Started</b></a> | <a href="#-student-research--google-ai-pro-edition"><b>Student Edition</b></a> | <a href="#-sponsor--support"><b>Sponsor</b></a> | <a href="#-how-to-contribute"><b>Contributing</b></a>
</p>

> **Disclaimer:** *This is an open source community-driven companion project and is not an officially sponsored Google product. It is built to extend and empower the Google Antigravity & Google AI developer ecosystem.*

---


## 📑 Table of Contents

- [Overview](#-overview)
- [Tribute to the Community & Google AI Pro](#-tribute-to-the-community--google-ai-pro)
- [The Problem: Why Antigravity Needs an Operator](#-the-problem-why-antigravity-needs-an-operator)
- [The Solution: Core Capabilities](#-the-solution-what-antigravity-operator-solves)
- [Landscape & Benchmark](#-landscape-how-agyo-compares)
- [Student, Research & Google AI Pro Edition](#-student-research--google-ai-pro-edition)
- [System Architecture (SRP, KISS, YAGNI, DRY)](#-system-architecture-srp-kiss-yagni-dry)
- [Getting Started & Installation](#-getting-started--installation)
- [CLI Reference & Usage](#-cli-reference--usage)
- [How to Contribute](#-how-to-contribute)
- [Sponsor & Support](#-sponsor--support)
- [Security & License](#-security--license)

---

## 🔭 Overview

**Antigravity Operator** (`agyo`) turns the raw power of Google Antigravity into an autonomous, safe, and persistent **Operating System Operator** (Claude Computer Use / OS Agent style).

By implementing the canonical **Outer Harness (Martin Fowler)** model, `agyo` provides:
1. **Deterministic Session Memory:** State persists directly in `.agents/session/` on disk (`state.md`, `decisions.md`, `todo.md`), eliminating context amnesia.
2. **Zero-Pollution Chrome Isolation:** Automatically launches and supervises a dedicated Chrome instance on port `9222` (`~/.gemini/antigravity-browser-profile`), keeping your personal browsing safe and untouched.
3. **Headless & Server Linux Parity:** Automatically detects missing graphical environments (`$DISPLAY` / `$WAYLAND_DISPLAY`) and activates robust server flags (`--headless=new`, `--disable-dev-shm-usage`, `--no-sandbox`).
4. **Single-Binary Portability:** Written in pure Go with `CGO_ENABLED=0` and embedded templates (`//go:embed`), producing a self-contained ~6MB executable requiring zero dependencies.

---

## 🎁 Tribute to the Community & Google AI Pro

> *"This project is an open engineering contribution to the developer community, students, and researchers worldwide, and a special thank you to **Google** for the transformative student access program through **Google AI Pro**."*

Our mission is to democratize high-end agentic engineering: enabling every student and software engineer to leverage 100% of their **Gemini Pro and Antigravity** quotas with professional discipline, zero token waste, and seamless portability across any Linux or macOS machine.

---

## 🔍 The Problem: Why Antigravity Needs an Operator

Google Antigravity provides state-of-the-art atomic tooling: arbitrary bash execution, surgical file edits, subagents, and Model Context Protocol (MCP) integrations.

**However, out-of-the-box, it is a raw-power engine lacking an operational harness:**
* **Competitors bundle proprietary sandboxes:** Tools like Claude Code, Devin, or Cursor enforce pre-configured guardrails. Antigravity provides atomic tools (`run_command`, `write_to_file`), leaving session governance, persistence, and OS lifecycle to the developer.
* **Operational Amnesia:** Without deterministic filesystem state, agents lose context across compaction windows and repeated sessions.
* **Runtime Friction:** Developers must manually configure CDP ports, isolate browser profiles, and debug headless Linux edge cases.

### The 4 Critical Failure Modes in Local AI Agents:
1. **The "Drunken Agent" Syndrome:** Agents executing unchecked commands, hallucinating paths, assuming code works without testing, and entering infinite retry loops burning API quota.
2. **Operational Amnesia & Context Drift:** As token limits are reached, agents forget earlier architectural agreements and repeat solved mistakes.
3. **Personal Browser Hijacking:** Agents interacting with the web using the user's personal browser profile, risking banking cookies, private tabs, or crashes.
4. **The macOS vs Linux Chasm:** Scripts developed on macOS failing on Linux servers, VPSs, WSL2, or Docker due to missing graphical displays (`$DISPLAY`), `/dev/shm` memory constraints, or sandbox permission errors.

---

## 💡 The Solution: What `antigravity-operator` Solves

| Capability | Engineering Implementation |
|---|---|
| **Outer Harness (Fowler)** | **Guide × Sensor:** Deterministic directives guide the model; automated tests (`go test`, linters, runtime probes) validate every change before completion. |
| **Brain & Hands Symbiosis** | **Google NotebookLM + Antigravity:** Connects NotebookLM as the grounded research brain and Antigravity as the autonomous coding hands, avoiding context window bloat. |
| **Filesystem Memory** | **`.agents/session/`:** Real-time state (`state.md`), architecture log (`decisions.md`), and task tracker (`todo.md`) persist across chat resets. |
| **Browser Supervision** | **Chrome DevTools Protocol (CDP):** Dedicated profile on port `9222`, PID tracking, and graceful shutdown (`agyo browser stop`). |
| **Auto-Headless Mode** | **Dynamic Display Probe:** Injects `--headless=new`, `--disable-dev-shm-usage`, and `--no-sandbox` automatically in server environments. |
| **Zero Runtime Deps** | **Pure Go (`CGO_ENABLED=0`):** Single static ~6MB binary containing all embedded rules and templates. |

---

## 🥊 Landscape: How `agyo` Compares

| Feature | Raw Bash Scripts | Claude Computer Use | Open-Interpreter | **Antigravity Operator (`agyo`)** |
|---|---|---|---|---|
| **Outer Harness Governance** | ❌ No | ❌ No | ❌ No | **✅ Native (Guide × Sensor)** |
| **Filesystem Session Memory** | ❌ No | ❌ No | ❌ No | **✅ Canonical `.agents/session/`** |
| **Grounded RAG Bridge (NotebookLM)** | ❌ No | ❌ No | ❌ No | **✅ Native (`agyo notebooklm` & MCP)** |
| **Isolated Browser Profile** | ❌ Uses personal | ⚠️ Heavy Docker | ❌ No | **✅ Dedicated Profile (`9222`)** |
| **macOS / Linux Parity** | ⚠️ Fragile | ⚠️ Docker-only | ⚠️ Dep conflicts | **✅ Native & Auto-Headless** |
| **Runtime Footprint** | Multi-tooling | Docker / APIs | Python / venv | **✅ Single Static Binary (~6MB)** |
| **Integrated Diagnostics (`doctor`)** | ❌ No | ❌ No | ❌ No | **✅ Built into CLI** |

---

## 🎓 Student, Research & Google AI Pro Edition

For computer science students and researchers leveraging academic benefits such as **Google AI Pro**, `agyo` is the ultimate productivity multiplier:

1. **Token Quota Conservation:** Prevents infinite retry loops and verbose repetitive code outputs, ensuring your Gemini Pro quota lasts the entire semester.
2. **Zero-Root Portability in University Labs (Linux):** University labs often run locked-down Linux machines without `sudo` access to install Docker or system packages. The static `agyo-linux-amd64` binary runs directly from user space (`~/`).
3. **Academic Logbook & Portfolio:** The `.agents/session/` folder preserves architectural rationales and algorithm trade-offs, turning daily coding into documented learning logs.
4. **Safe Sandbox:** Isolated Chrome automation protects personal university credentials and institutional logins.
5. **Symbiosis with Google NotebookLM (Zero-Context-Bloat Grounded RAG):** Ingest entire textbooks, papers, and assignment specs into NotebookLM. With `agyo notebooklm`, Antigravity queries authoritative sources with exact citations before writing code.

### 🎁 Bonus Student Skills Included (`skills/`):
This repository includes 3 canonical skills out-of-the-box:
* **`feynman-code-tutor`:** Senior tutor based on the Feynman Technique. Explains complex algorithms, data structures, and Big-O using real-world analogies and comprehension checkpoints.
* **`student-study-planner`:** Breaks down complex college syllabi, final projects, and technical interview prep into focused sprint cycles (20% theory, 80% deliberate coding).
* **`token-budget-guard`:** Surgical token optimizer ensuring context efficiency and zero repetitive code waste.

---

## 🏛️ System Architecture (SRP, KISS, YAGNI, DRY)

```text
antigravity-operator/
├── cmd/agyo/                 # CLI entrypoint (main.go)
├── internal/
│   ├── platform/             # SRP: OS detection, X11/Wayland check, Chrome binary resolution
│   ├── session/              # SRP: .agents/session/ scaffold & gitignore protection
│   ├── profile/              # SRP: Chrome lifecycle management, PID tracking & CDP port
│   ├── watcher/              # SRP: Brain streaming, subagent tree hierarchy & OS notifications
│   ├── exporter/             # SRP: Consolidated session report generator (Markdown & HTML)
│   ├── dashboard/            # SRP: Pure-Go embedded HTTP server, web UI & REST API
│   ├── completion/           # SRP: Shell autocompletion generator (Bash, Zsh, Fish)
│   ├── hook/                 # SRP: Git pre-commit continuity sensor & safeguards
│   ├── installer/            # SRP: Idempotent rule and MCP manifesto synchronization
│   ├── notebook/             # SRP: Native Google NotebookLM integration & stdio MCP server
│   └── doctor/               # SRP: Machine diagnostic computational sensors
├── templates/                # Embedded static assets via //go:embed (zero external deps)
│   ├── rules/                # Canonical Session Agent rules
│   ├── session/              # Templates for state.md, decisions.md, and todo.md
│   └── mcps/                 # Default MCP servers manifest (DevTools, Playwright)
├── Formula/                  # Official Homebrew package formula (agyo.rb)
├── agents/                   # Specialized AI personas (operator-architect, cdp-engineer, qa-sentinel)
├── skills/                   # Bonus student skills (feynman tutor, study planner, token guard)
├── docs/                     # Architecture specs & community launch articles (TabNews, LinkedIn)
├── .github/                  # CI/CD workflows, release automation & community issue templates
├── scripts/                  # Universal installer (install.sh) and developer setup (setup-dev.sh)
├── AUTHORS                   # Project authors
├── CONTRIBUTORS              # Project contributors
├── CODE_OF_CONDUCT.md        # Google Open Source Community Guidelines
├── SECURITY.md               # Responsible vulnerability disclosure policy
├── PRIVACY.md                # Zero-telemetry local-first privacy policy (LGPD & GDPR)
├── CONTRIBUTING.md           # Contribution guide and testing protocol
```

> 📖 **Comprehensive Engineering Specification:**  
> For an in-depth breakdown of every subsystem, RFC 6455 WebSocket CDP implementation, $O(1)$ memory algorithms, and design decisions, read the [System Specification Index (docs/spec/)](docs/spec/README.md).

---

---

## ⚡ Getting Started & Installation

### Option 1: Universal One-Liner (Zero-Config / Recommended)
Install pre-compiled static binaries directly on macOS or Linux (no Go required):
```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash
```

### Option 2: Homebrew (macOS & Linuxbrew)
```bash
brew install tiagovilasboas/tap/agyo
```

### Option 3: Build from Source (Go 1.22+)
```bash
git clone https://github.com/tiagovilasboas/antigravity-operator.git
cd antigravity-operator
make build
make install
```

---

## 🚀 CLI Reference & Usage

### 1. Environment Diagnostics & Self-Healing (`doctor`)
Inspects system readiness across OS, Git, Chrome, Node/NPX, and Harness connections, with support for automatic environment self-healing:
```bash
# Standard interactive diagnostics:
agyo doctor

# Machine-readable output for scripts and IDE extensions:
agyo doctor --json

# Self-healing sensor: automatically repair missing session files and corrupted state:
agyo doctor --fix
```

### 2. Scaffold Operational Memory (`init`)
Creates the `.agents/session/` memory structure in your current project:
```bash
cd my-project
agyo init
```

Files created:
- `.agents/session/state.md` (Real-time objective and status)
- `.agents/session/decisions.md` (Architecture log and trade-offs)
- `.agents/session/todo.md` (Task tracker)
- `.agents/.gitignore` (Protects runtime logs and sensitive credentials)

### 3. Inspect & Manage Session Memory (`session`)
Monitor your agent's active progress, stream reasoning in real time, or archive completed missions:
```bash
# View active mission objective, phase, and task completion percentage:
agyo session status

# Stream active agent reasoning and tool executions live with desktop notifications:
agyo session watch

# Inspect active subagent tree (invoke_subagent & inter-agent messages):
agyo session watch --once --steps 10 --tree

# Export consolidated mission report in Markdown or standalone responsive HTML:
agyo session export --format=markdown
agyo session export --format=html --out=session-report.html

# Compact completed tasks to avoid context bloat (archives older tasks with rollup):
agyo session compact
agyo session compact --dry-run
agyo session compact --threshold 5 --keep 3

# Archive completed session to historical log and reset templates for the next task:
agyo session archive

# List all archived historical sessions:
agyo session list

# Restore a past archived session into active memory (creates safety backup automatically):
agyo session restore session-2026-10-01-113140.md
agyo session restore latest
```

### 4. Filesystem Safety Net & Atomic Checkpoints (`checkpoint` & `rollback`)
Protect your repository against hallucinated or destructive agent refactorings. Take an atomic snapshot before an agent begins risky edits, and rollback instantaneously if things go sideways:
```bash
# Create an atomic snapshot before starting a complex multi-file edit:
agyo checkpoint "pre-refactor" --desc="Before database schema migration"

# List all saved checkpoints:
agyo checkpoint --list

# Instant panic button: Safely undo agent changes and restore exact working tree:
agyo rollback
agyo rollback chk-20261001-113000
```

### 5. Token Waste Protection (`.agentignore`)
When running `agyo init`, a pre-configured `.agentignore` blacklist is automatically scaffolded to prevent agents from loading heavyweight dependencies into prompt context:
- Excludes `node_modules/`, `vendor/`, `dist/`, `.git/`
- Excludes lockfiles (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`)
- Excludes large dumps, datasets, minified bundles (`*.min.js`), and local `.env` files

### 6. Live Web Dashboard & Local Inspector (`dashboard`)
Spawns a pure-Go zero-dependency web dashboard on `http://127.0.0.1:8080` with dark-mode UI, live session progress, doctor diagnostics, active Chrome tabs, and live activity feeds:
```bash
# Launch dashboard and automatically open default browser:
agyo dashboard

# Launch on a custom port without auto-opening browser:
agyo dashboard --port 8090 --open=false
```

### 5. Manage Isolated Chrome Lifecycle & CDP (`browser`)
Full process supervision with PID tracking, graceful shutdown, and pure-Go Chrome DevTools Protocol inspection:
```bash
# Launch isolated Chrome on port 9222 (Desktop GUI):
agyo browser start

# Launch on a custom port or force headless mode (automatic on headless Linux/VPS):
agyo browser start --port 9223
agyo browser start --headless

# Check status, port readiness, and Chrome PID:
agyo browser status

# Pure-Go DevTools Protocol (CDP) inspection (zero Node/Python scripts needed):
agyo browser tabs                    # List all open tabs and target IDs
agyo browser open https://github.com # Open a URL in a new tab
agyo browser close <targetId>        # Close target tab
agyo browser eval "document.title"   # Evaluate JS expression in active tab
agyo browser shot screenshot.png     # Capture PNG screenshot via CDP

# Gracefully terminate isolated Chrome instance (SIGTERM):
agyo browser stop
```

### 6. Native Google NotebookLM Integration (`notebooklm`)
Connects Google Antigravity agents and terminal sessions directly to your **Google NotebookLM** notebooks and grounded sources with automated CDP session handling (zero Python/Node dependencies):
```bash
# Check connection and authentication status with Google NotebookLM:
agyo notebooklm status

# Launch isolated Chrome and open Google NotebookLM (persisted Google profile):
agyo notebooklm open

# List all available notebooks in your active Google account:
agyo notebooklm list

# Ask a grounded question to a specific notebook:
agyo notebooklm ask <notebook-id> "What is the architecture described in the specifications?"

# Push local markdown or text files as sources to a notebook:
agyo notebooklm push <notebook-id> docs/spec/ARCHITECTURE.md

# Run stdio Model Context Protocol (MCP) server for agents (Antigravity / Cursor / Claude):
agyo notebooklm mcp
```
*(Short aliases supported: `agyo notebook` and `agyo nblm`)*

### 7. Git Pre-Commit Continuity Hook (`hook`)
Installs an automated session sensor into `.git/hooks/pre-commit` to prevent committing code without updating session objectives and task progress:
```bash
# Install hook in current repository (or specific target dir):
agyo hook install

# Uninstall hook when needed:
agyo hook uninstall
```

### 7. Sync Rules, Skills, and MCP Manifestos (`sync`)
Provisions canonical rules and automation manifests into Google Antigravity:
```bash
agyo sync
```

### 8. Shell Autocompletion (`completion`)
Generate command and flag autocompletion for Bash, Zsh, or Fish:
```bash
# Zsh (add to ~/.zshrc):
source <(agyo completion zsh)

# Bash (add to ~/.bashrc):
source <(agyo completion bash)

# Fish:
agyo completion fish | source
```

### 9. Project Manifesto (`about`)
```bash
agyo about
```

---

## 🤝 How to Contribute

We welcome contributions from engineers, students, and open-source enthusiasts! Whether fixing bugs, improving docs, or adding new skills, your help is appreciated.

### 5-Step Contribution Workflow:
1. **Fork & Clone:**
   ```bash
   git clone https://github.com/tiagovilasboas/antigravity-operator.git
   cd antigravity-operator
   ```
2. **Automated Dev Setup (installs pre-commit hooks):**
   ```bash
   ./scripts/setup-dev.sh
   ```
3. **Create a Feature Branch:**
   ```bash
   git checkout -b feat/your-awesome-feature
   ```
4. **Run Sensors & Tests:**
   ```bash
   go vet ./...
   go test -v -race ./...
   make build
   ./bin/agyo doctor
   ```
5. **Open a Pull Request:** Follow [Conventional Commits](https://www.conventionalcommits.org/) in English and submit your PR.

### 💡 High-Impact Contribution Ideas:
* **Student Skills:** Add new learning or research skills to `skills/` (e.g., dynamic programming tutor, thesis literature reviewer).
* **MCP Integrations:** Create templates for popular community Model Context Protocol servers in `templates/mcps/`.
* **Linux Distribution Testing:** Verify and document compatibility on Arch Linux, Alpine, Fedora, or NixOS.

Please review our [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [AUTHORS](AUTHORS), and [CONTRIBUTORS](CONTRIBUTORS).

---

## 💖 Sponsor & Support

This project is an independent open-source initiative built to empower students and developers utilizing Google AI and Google Antigravity.

If `antigravity-operator` saved you time, protected your API token quota, or helped your academic research:

* ⭐ **Star the Repository:** The easiest and most effective way to help the project reach more students and catch Google's attention.
* 💖 **Sponsor on GitHub:** Help fund continuous integration compute and multi-OS testing instances via [GitHub Sponsors](https://github.com/sponsors/tiagovilasboas).
* 🗣️ **Share with Your Community:** Post about your experience on LinkedIn, X/Twitter, Discord, or university study groups.

---

## 🔒 Security, Privacy & License

* **Security Policy:** Refer to [SECURITY.md](SECURITY.md) for responsible vulnerability disclosure guidelines.
* **Privacy & LGPD Compliance:** Refer to [PRIVACY.md](PRIVACY.md) for our zero-telemetry, local-first data protection policy (LGPD & GDPR).
* **License:** Distributed under the [MIT](LICENSE) License.

