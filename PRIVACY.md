# Privacy Policy & Data Protection (LGPD & GDPR Compliance)

Antigravity Operator (`agyo`) is committed to the highest standards of data privacy, user sovereignty, and security. This document details our architecture, zero-telemetry guarantee, and adherence to the Brazilian General Data Protection Law (**LGPD - Lei Federal nº 13.709/2018**) and the General Data Protection Regulation (**GDPR - Regulation (EU) 2016/679**).

---

## 1. Core Principle: Local-First, No Telemetry

- **No telemetry:** the `agyo` binary contains no analytics, tracking beacons or pingbacks, and sends nothing about you, your code or your sessions to the maintainer or any third party.
- **No cloud storage:** everything `agyo` writes stays on your machine (listed below).
- **Network use is limited and listed:** the only traffic that leaves your machine comes from the installer, the MCP servers it configures, and the Chrome instance it launches. See the table below. The `agyo` binary itself only opens loopback connections (`127.0.0.1`).

### 1.1. What touches the network

| Component | Destination | When |
| :--- | :--- | :--- |
| `scripts/install.sh` | `api.github.com` (latest release tag), `github.com` (release archive and `checksums.txt`, or `git clone` when building from source) | Only when you run the installer |
| Homebrew formula | `github.com` (source tarball), Go module proxy for the build | Only on `brew install` / `brew upgrade` |
| MCP servers in `templates/mcps/default-servers.json` (`chrome-devtools-mcp`, `@executeautomation/playwright-mcp-server`, pinned versions) | `registry.npmjs.org` via `npx` to download the package; then whatever the server itself does | When your AI client (e.g. Antigravity) starts them, not when `agyo` runs |
| Chrome launched by `agyo browser` | Whatever Chrome and the pages you or the agent open contact (updates, sites) | While that browser runs |

### 1.2. What `agyo` reads locally

| Data | Path or source | Used by |
| :--- | :--- | :--- |
| Antigravity session transcripts | `~/.gemini/antigravity/brain/*/.system_generated/logs/transcript.jsonl` (CLI) and `~/.gemini/brain/*/…/transcript.jsonl` (dashboard) | `session watch`, `session export`, `dashboard` (dashboard events show only type, step, tool and status) |
| Session memory | `.agents/session/` in your project (`state.md`, `todo.md`, `decisions.md`, `checkpoints.json`) | `session`, `checkpoint`, `rollback`, `dashboard` |
| Git metadata | `git config user.name` / `user.email`, the project's repository | `doctor`, `checkpoint`, `rollback`, `hook` |
| Chrome tabs | CDP on `127.0.0.1:9222` (tab titles and URLs of the isolated profile) | `browser`, `dashboard` |
| Host facts | OS, architecture, display, `npx` presence, running `antigravity` processes | `doctor` |

The dashboard (`agyo dashboard`) serves this data over HTTP on `127.0.0.1` only. It does not send it anywhere else.

### 1.3. What `agyo` writes locally

`.agents/session/` and `.agents/.gitignore` in your project, `~/.gemini/antigravity/{rules,mcp}/`, `~/.gemini/config/skills/agyo/`, the isolated browser profile `~/.gemini/antigravity-browser-profile`, and a git pre-commit hook when you run `agyo hook`.

---

## 2. Adherence to LGPD Principles (Art. 6º, Lei 13.709/2018)

| LGPD Principle | How `agyo` Complies |
| :--- | :--- |
| **Finalidade (Purpose)** | Data processing is strictly confined to local session management, developer ergonomics, and tooling orchestration. |
| **Adequação (Suitability)** | Only operational metadata (task checklists, architectural notes, run status) is recorded. |
| **Necessidade (Data Minimization)** | No Personally Identifiable Information (PII), biometric data, sensitive credentials, or payment data is required or collected. |
| **Livre Acesso & Transparência (Access & Transparency)** | All generated files (`.agents/session/state.md`, `decisions.md`, `todo.md`) are human-readable Markdown files stored locally in your project folder. |
| **Segurança & Prevenção (Security & Prevention)** | Dedicated browser profile isolation prevents the agent from reading or leaking your personal browser cookies, saved passwords, or private bookmarks. |
| **Responsabilização (Accountability)** | Complete auditability through local git versioning and clear file structures. |

---

## 3. Data Sovereignty & Subject Rights (Art. 18 LGPD)

Because all state is stored locally on your machine, you retain complete sovereignty over your data:

- **Immediate Elimination (Direito de Exclusão):** `agyo session archive` moves the current session into `.agents/session/archive/`; to erase everything, delete the local folders:
  ```bash
  rm -rf .agents/session
  rm -rf ~/.gemini/antigravity-browser-profile
  ```
- **Portability (Portabilidade):** Since data is plain-text Markdown and standard JSON, you can freely transfer, backup, or inspect it without proprietary lock-in.

---

## 4. Browser Isolation & Credential Protection

When launching automated browser sessions via `agyo browser`:

1. **Isolated User Data Directory:** The browser runs inside `~/.gemini/antigravity-browser-profile`, separate from your primary Chrome user profile (`Default` / `Profile 1`).
2. **Cookie & History Segregation:** Your personal browsing history, enterprise SSO cookies, and personal Google accounts remain completely isolated from automation tasks.
3. **Remote Debugging Safety:** The Chrome DevTools Protocol (CDP) port is bound to `127.0.0.1:9222` (loopback only) and is never exposed to external network interfaces.

---

## 5. Best Practices for Developers Handling Sensitive Data

While `agyo` itself collects no personal data, developers using AI agents on enterprise repositories should observe the following guidelines:

1. **Do Not Commit Real PII:** Never paste customer CPFs, real phone numbers, credit card numbers, or passwords into session notes (`state.md` / `decisions.md`).
2. **Use Synthetic Data:** When testing database queries or browser flows with the agent, use mock fixtures, factory generators, or anonymized datasets.
3. **Exclude Session Files When Required:** If your project guidelines prohibit committing session state to version control, add `.agents/session/` to your repository's `.gitignore`.

---

## 6. Contact & Data Protection Officer (DPO)

For questions, security concerns, or privacy inquiries regarding Antigravity Operator:

- **Maintainer:** Tiago Vilas Boas
- **Email:** `tcarvalhovb@gmail.com`
- **Security Policy:** [SECURITY.md](SECURITY.md)
