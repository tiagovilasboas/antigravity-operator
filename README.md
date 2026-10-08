# Antigravity Operator (`agyo`)

<p align="center">
  <a href="https://github.com/tiagovilasboas/antigravity-operator/releases"><img src="https://img.shields.io/github/v/release/tiagovilasboas/antigravity-operator?style=flat-square&logo=github&color=blue" alt="Release" /></a>
  <a href="https://github.com/tiagovilasboas/antigravity-operator/actions"><img src="https://img.shields.io/github/actions/workflow/status/tiagovilasboas/antigravity-operator/ci.yml?style=flat-square&logo=githubactions&logoColor=white&label=CI" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/tiagovilasboas/antigravity-operator"><img src="https://goreportcard.com/badge/github.com/tiagovilasboas/antigravity-operator?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-000000?style=flat-square&logo=apple&logoColor=white" alt="Platform" />
  <img src="https://img.shields.io/badge/Architecture-Single%20Binary%20(No%20CGO)-success?style=flat-square" alt="Binary" />
  <img src="https://img.shields.io/badge/Pattern-Fowler%20Outer%20Harness-blueviolet?style=flat-square" alt="Pattern" />
  <img src="https://img.shields.io/badge/Sponsor-GitHub%20Sponsors-ea4aaa?style=flat-square&logo=githubsponsors" alt="Sponsor" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

**Antigravity Operator (`agyo`)** is the open source session runtime and outer harness for Google Antigravity. It bridges **Google NotebookLM (the Research Brain)** to **Antigravity (the Execution Hands)** — allowing developers and students to ingest YouTube video playlists, technical books, and academic papers to generate deep technical articles, study guides, and verified code implementations with zero context bloat and ~99% token savings. Governed by Martin Fowler's Outer Harness model (Guide × Sensor), on-disk session memory (`.agents/session/`), atomic git checkpoints with instant rollback, and pure-Go CDP browser supervision.

Agents: read [AGENTS.md](AGENTS.md) first.

## Quickstart

Install a release binary (macOS/Linux, verified against `checksums.txt`) or use Homebrew:

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash
# or
brew install tiagovilasboas/tap/agyo
```

Then, inside any git repo:

```bash
agyo init             # creates .agents/session/ and .agentignore
agyo session status   # objective, phase and task progress
agyo dashboard        # local UI on http://127.0.0.1:8080 (add --open=false on a headless box)
```

None of these need Chrome. Chrome is only needed for `agyo browser ...`. The activity panel fills in once Antigravity writes a session transcript. The Homebrew Formula builds from source and can trail the latest release; other options are in [Getting Started](#-getting-started--installation).

![agyo dashboard right after agyo init, with a sample transcript in the activity panel](docs/assets/dashboard.png)

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
- [System Architecture (SRP, KISS, YAGNI, DRY)](#️-system-architecture-srp-kiss-yagni-dry)
- [AI-Assisted Development & Agent Ecosystem](#-ai-assisted-development--agent-ecosystem)
- [Getting Started & Installation](#-getting-started--installation)
- [CLI Reference & Usage](#-cli-reference--usage)
- [Canonical Operating Principles](#️-canonical-operating-principles)
- [How to Contribute](#-how-to-contribute)
- [Sponsor & Support](#-sponsor--support)
- [Security, Privacy & License](#-security-privacy--license)

---

## 🔭 Overview

**Antigravity Operator** (`agyo`) wraps Google Antigravity sessions with on-disk memory, git checkpoints, a local dashboard and a supervised Chrome profile. It does not sandbox the agent: commands the agent runs still have your user's permissions.

By implementing the canonical **Outer Harness (Martin Fowler)** model, `agyo` provides:
1. **Brain & Hands Symbiosis (Grounded Multimodal RAG):** Connects to **Google NotebookLM** via CDP and MCP to ground coding and study sessions on YouTube video playlists, technical books, and academic papers with exact timestamps and citations — costing zero tokens in your agent context window.
2. **Deterministic Session Memory:** State persists directly in `.agents/session/` on disk (`state.md`, `decisions.md`, `todo.md`), eliminating context amnesia.
3. **Separate Chrome Profile:** Launches and supervises a dedicated Chrome instance on port `9222` (`~/.gemini/antigravity-browser-profile`), so agent browsing stays out of your personal profile. This is a separate profile, not a sandbox: anything that can reach the DevTools port controls that browser.
4. **Headless & Server Linux Parity:** Automatically detects missing graphical environments (`$DISPLAY` / `$WAYLAND_DISPLAY`) and activates robust server flags (`--headless=new`, `--disable-dev-shm-usage`, `--no-sandbox`).
5. **Single-Binary Portability:** Written in pure Go with `CGO_ENABLED=0` and embedded templates (`//go:embed`), producing a single self-contained executable with no runtime dependencies.

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
| **Outer Harness (Fowler)** | **Guide × Sensor:** Deterministic directives guide the model; the rules ask the agent to run tests (`go test`, linters, runtime probes) before calling a change done. `agyo` provides the rules and sensors; it does not enforce them. |
| **Brain & Hands Symbiosis** | **Google NotebookLM + Antigravity:** Connects NotebookLM as the grounded research brain and Antigravity as the autonomous coding hands, avoiding context window bloat. |
| **Filesystem Memory** | **`.agents/session/`:** Real-time state (`state.md`), architecture log (`decisions.md`), and task tracker (`todo.md`) persist across chat resets. |
| **Browser Supervision** | **Chrome DevTools Protocol (CDP):** Dedicated profile on port `9222`, PID tracking, and graceful shutdown (`agyo browser stop`). |
| **Auto-Headless Mode** | **Dynamic Display Probe:** Injects `--headless=new`, `--disable-dev-shm-usage`, and `--no-sandbox` automatically in server environments. |
| **Zero Runtime Deps** | **Pure Go (`CGO_ENABLED=0`):** Single static binary containing all embedded rules and templates. |

---

## 🥊 Landscape: How `agyo` Compares

| Feature | Raw Bash Scripts | Claude Computer Use | Open-Interpreter | **Antigravity Operator (`agyo`)** |
|---|---|---|---|---|
| **Outer Harness Governance** | ❌ No | ❌ No | ❌ No | **✅ Native (Guide × Sensor)** |
| **Filesystem Session Memory** | ❌ No | ❌ No | ❌ No | **✅ Canonical `.agents/session/`** |
| **Grounded RAG Bridge (NotebookLM)** | ❌ No | ❌ No | ❌ No | **✅ Native (`agyo notebooklm` & MCP)** |
| **Isolated Browser Profile** | ❌ Uses personal | ⚠️ Heavy Docker | ❌ No | **✅ Dedicated Profile (`9222`)** |
| **macOS / Linux Parity** | ⚠️ Fragile | ⚠️ Docker-only | ⚠️ Dep conflicts | **✅ Native & Auto-Headless** |
| **Runtime Footprint** | Multi-tooling | Docker / APIs | Python / venv | **✅ Single Static Binary** |
| **Integrated Diagnostics (`doctor`)** | ❌ No | ❌ No | ❌ No | **✅ Built into CLI** |

---

## 🎓 Student, Research & Google AI Pro Edition

For computer science students and researchers leveraging academic benefits such as **Google AI Pro**, `agyo` is the ultimate productivity multiplier:

1. **Token Quota Conservation:** `.agentignore` keeps dependencies, lockfiles and dumps out of the prompt, and `agyo session compact` keeps `todo.md` short.
2. **Zero-Root Portability in University Labs (Linux):** University labs often run locked-down Linux machines without `sudo` access to install Docker or system packages. The static `agyo-linux-amd64` binary runs directly from user space (`~/`).
3. **Academic Logbook & Portfolio:** The `.agents/session/` folder preserves architectural rationales and algorithm trade-offs, turning daily coding into documented learning logs.
4. **Separate Browser Profile:** Agent browsing runs in its own Chrome profile, away from your personal logins. It is not a sandbox.
5. **Symbiosis with Google NotebookLM (Zero-Context-Bloat Grounded RAG):** Ingest entire textbooks, papers, assignment specs, and YouTube videos into NotebookLM. With `agyo notebooklm`, Antigravity queries authoritative sources with exact citations before writing code.

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
└── Makefile                  # Native build, lint, coverage and cross-compilation targets
```

> 📖 **Comprehensive Engineering Specification:**  
> For an in-depth breakdown of every subsystem, RFC 6455 WebSocket CDP implementation, $O(1)$ memory algorithms, and design decisions, read the [System Specification Index (docs/spec/)](docs/spec/README.md).

---

## 🤖 AI-Assisted Development & Agent Ecosystem

The project follows an **Agent-as-Code** approach:
- **`AGENTS.md`:** Operating contract for any AI assistant (Antigravity, Cursor, Claude, Copilot): Go conventions, the sensor checklist and commit rules.
- **Specialist roster (`agents/`):**
  - **`operator-architect`:** Guardian of OS behavior, macOS/Linux parity and KISS/YAGNI.
  - **`cdp-engineer`:** Chrome DevTools Protocol, browser flags and debugging sockets.
  - **`qa-sentinel`:** Automated tests and regression sensors.

---

## ⚡ Getting Started & Installation

### Option 1: Universal One-Liner (Zero-Config / Recommended)
Install pre-compiled static binaries directly on macOS or Linux (no Go required):
```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/antigravity-operator/main/scripts/install.sh | bash
```
Archives are also on the [Releases](https://github.com/tiagovilasboas/antigravity-operator/releases) page. From v0.4.6 on they carry a build provenance attestation; see [SECURITY.md](SECURITY.md) to verify one with `gh attestation verify`.

### Option 2: Homebrew (macOS & Linuxbrew)
```bash
brew install tiagovilasboas/tap/agyo
```

### Option 3: Build from Source (Go 1.27+, see go.mod)
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
- `.agentignore` (Token blacklist: excludes `node_modules/`, `vendor/`, lockfiles and dumps; see section 5)

### 3. Inspect & Manage Session Memory (`session`)
Monitor your agent's active progress, stream reasoning in real time, or archive completed missions:
```bash
# View active mission objective, phase, and task completion percentage (supports --json):
agyo session status
agyo session status --json

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

# List all archived historical sessions (supports --json):
agyo session list
agyo session list --json

# Restore a past archived session into active memory (creates safety backup automatically):
agyo session restore session-2026-10-01-113140.md
agyo session restore latest
```

### 4. Filesystem Safety Net & Atomic Checkpoints (`checkpoint` & `rollback`)
Take a snapshot before an agent starts risky edits and roll back if things go sideways. Rollback restores tracked files, removes files created after the checkpoint, and keeps `.agents/` and files that were already untracked (they are kept, not snapshotted):
```bash
# Create an atomic snapshot before starting a complex multi-file edit:
# (flags go before the name)
agyo checkpoint --desc="Before database schema migration" pre-refactor

# List all saved checkpoints (supports --json):
agyo checkpoint --list
agyo checkpoint --list --json

# Undo agent changes back to the latest (or a given) checkpoint:
agyo rollback
agyo rollback chk-20261001-113000
```
If you committed after the checkpoint, `rollback` moves the current branch back to the checkpoint commit (it refuses on a detached HEAD or another branch). It saves uncommitted changes to a stash first and prints how to undo (`git reset --hard <old>`, also in `git reflog`). `.agents/` is never changed.

### 5. Token Waste Protection (`.agentignore`)
When running `agyo init`, a pre-configured `.agentignore` blacklist is automatically scaffolded to prevent agents from loading heavyweight dependencies into prompt context:
- Excludes build output and dependencies (`node_modules/`, `vendor/`, `dist/`, `build/`, `target/`)
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

### 7. Manage Isolated Chrome Lifecycle & CDP (`browser`)
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

### 8. Native Google NotebookLM Integration (`notebooklm`)
Bridges Google Antigravity and your terminal directly to **Google NotebookLM** via pure-Go DevTools CDP and a stdio Model Context Protocol (MCP) server (zero Python or Node dependencies).

#### 💡 The Killer Workflow: Videos → Technical Articles & Verified Code
The greatest potential of this integration is transforming passive consumption (long YouTube videos, lectures, documentation, textbooks) into **grounded technical articles, study guides, and verified code implementations** in your local workspace:

```text
[ YouTube Video / Course / Paper ]
                │
                ▼
     [ Google NotebookLM ] ── (Multimodal index, official transcript & timestamped citations at 0 token cost)
                │
                ▼ (Pure-Go CDP / RFC 6455 WebSocket / stdio MCP)
     [ agyo notebooklm / MCP ] ── (`notebooklm_ask` or `agyo notebooklm ask`)
                │
                ▼
     [ Google Antigravity ] ── (Synthesizes deep technical articles & verified code implementations)
                │
                ▼
[ ~/Estudos/artigos/*.md + Unit Tests ] ── (Perennial study assets, documented architecture & tested code)
```

1. **Ingest without Token Penalty:** Paste YouTube video links, PDFs, or documentation into a NotebookLM notebook. NotebookLM ingests full transcripts, timestamps, and diagrams into Gemini 1.5 Pro multimodal embeddings at **zero token cost** to your agent.
2. **Grounded Query via CDP/MCP:** Antigravity queries the notebook with targeted prompts (`agyo notebooklm ask <id> "..."` or MCP `notebooklm_ask`).
3. **Generate Structured Articles & Study Guides:** Antigravity writes comprehensive technical articles, Feynman-style tutorials, or architectural ADRs into your study folder (e.g. `~/Estudos/artigos/`).
4. **Implement & Validate Code:** The agent turns theoretical concepts into tested implementations, validating them end-to-end with unit tests and browser checks.

#### 💰 Token Economics (FinOps): ~99.2% Quota Savings
* **Without NotebookLM:** Storing a 1-hour YouTube transcript (~25,000 tokens) or textbook chapters (~100,000 tokens) directly in the agent context window burns **1,500,000+ input tokens** across 15 conversation turns, triggering TPM rate limits and severe *Lost in the Middle* amnesia.
* **With `agyo notebooklm`:** The dense knowledge base is externalized. A 60-token query returns a ~500-token cited response. Total 15-turn consumption is just ~12,000 tokens — **an effective ~99.2% token savings**.

```bash
# 1. Check connection and authentication status with Google NotebookLM:
agyo notebooklm status

# 2. Launch isolated Chrome directly into Google NotebookLM (session preserved in profile):
agyo notebooklm open

# 3. List all available notebooks in your active Google account:
agyo notebooklm list

# 4. Ask a grounded question with exact citations to a notebook:
agyo notebooklm ask <notebook-id> "Explain the consensus algorithm from the video and provide Go code"

# 5. Push local markdown notes, code or study summaries back to the notebook:
agyo notebooklm push <notebook-id> ~/Estudos/artigos/distributed-systems.md

# 6. Run stdio Model Context Protocol (MCP) server for Antigravity, Claude Code or Cursor:
agyo notebooklm mcp
```
*(Short aliases supported: `agyo notebook` and `agyo nblm`)*

### 9. Git Pre-Commit Continuity Hook (`hook`)
Installs a `.git/hooks/pre-commit` script that prints the session status when `agyo` is on your PATH. When `todo.md` has 5 or more done tasks, it also runs `agyo session compact` and then `git add .agents/session/`, so the compacted session files are staged into that same commit. It is a reminder, not a gate: it always exits 0 and never blocks a commit:
```bash
# Install hook in current repository (or specific target dir):
agyo hook install

# Uninstall hook when needed:
agyo hook uninstall
```

### 10. Sync Rules, Skills, and MCP Manifestos (`sync`)
Provisions canonical rules and automation manifests into Google Antigravity:
```bash
agyo sync
```
`sync` never overwrites an existing MCP manifest (`~/.gemini/antigravity/mcp/default-servers.json`). `agyo doctor` warns when that file has unpinned `npx` packages or differs from the built-in template. To back it up (`default-servers.json.bak-<UTC timestamp>`) and rewrite it:
```bash
agyo sync --update-mcp
```

### 11. Shell Autocompletion (`completion`)
Generate command and flag autocompletion for Bash, Zsh, or Fish:
```bash
# Zsh (add to ~/.zshrc):
source <(agyo completion zsh)

# Bash (add to ~/.bashrc):
source <(agyo completion bash)

# Fish:
agyo completion fish | source
```

### 12. Project Manifesto (`about`)
```bash
agyo about
```

---

## 🛡️ Canonical Operating Principles

When Antigravity runs under `agyo`, it follows 5 rules:
1. **Investigate first:** Look for facts in the terminal, browser and logs before asking trivial questions.
2. **Multi-tool orchestration:** Identify -> Investigate -> Implement -> Test -> Validate in the browser.
3. **Rigorous validation:** A task is done only when the result is validated end to end with evidence.
4. **Separate profile:** Never drive the user's personal Chrome.
5. **Concise communication:** Direct, technical and grounded in data.

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

Please review our [CONTRIBUTING.md](CONTRIBUTING.md) (what gets accepted), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [AUTHORS](AUTHORS), and [CONTRIBUTORS](CONTRIBUTORS).

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

