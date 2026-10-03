# Contributing to Antigravity Operator (`agyo`)

Thank you for your interest in contributing to **Antigravity Operator**! This project is an open-source initiative designed to bring a robust, production-grade **Outer Harness** and session persistence engine to Google Antigravity and the broader Google AI developer ecosystem.

---

## 🧭 Code of Conduct & Core Philosophy

We follow the **Martin Fowler Outer Harness** model:
1. **Guide × Sensor:** Provide deterministic instructions (Guides) and automated tests (Sensors) before shipping.
2. **KISS & YAGNI:** Keep it simple, do not over-engineer. Avoid premature abstractions and unnecessary dependencies.
3. **Zero CGO:** Everything must compile statically (`CGO_ENABLED=0`) across Darwin (macOS) and Linux.
4. **No Assumptions:** Write code backed by real OS inspections and verified unit tests.

---

## ✅ What Gets Accepted

`agyo` is an outer harness for Google Antigravity: it keeps sessions scoped, evidence-backed, and recoverable. Contributions that fit are the ones that make an operator's life better on the problems in the [README](README.md#-the-problem-why-antigravity-needs-an-operator): **supervision** (browser/CDP, doctor sensors), **observability** (watcher, the embedded dashboard), and **session persistence** (`.agents/session/`, checkpoints).

**Out of scope** (we'll politely decline these):
- Exposing prompts, transcript content, or commands in the API or UI without redaction.
- Reading undocumented internal Antigravity logs or files without an approved proposal first.
- Growing the dashboard beyond the simple embedded single-file UI described in [spec 06](docs/spec/06-OBSERVABILIDADE-WATCHER-E-DASHBOARD.md): no frameworks, build steps, or CDN assets.
- New dependencies (`go.mod` stays on the standard library).
- Features without a clear, real pain behind them.

**Size:** one subject per PR. Around **400 lines of code** is the reference (tests and generated files don't count). Bigger than that? Slice it into a sequence of PRs, or explain in the description why it can't be split.

**Issue first:** for a new feature or any UI/API change, open an issue and agree on scope before writing code. Bug fixes and docs can go straight to a PR.

**Privacy by default:** everything stays local and the dashboard listens on localhost only. Read [PRIVACY.md](PRIVACY.md) before touching anything that reads or shows session data.

**Keep it lean:** only what delivers clear operator value. Leave out nice-to-have, cosmetic or unrequested changes, and don't let scope grow along the way. When in doubt, ask in an issue first.

**AI-assisted contributions are welcome.** You review and own every line you submit, though: please don't send bundles of generated features. Small, understood changes get merged fast.

---

## 🛠️ Development Setup

### Prerequisites
- Go 1.22+
- Git
- Google Chrome or Chromium (optional for runtime browser testing)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/tiagovilasboas/antigravity-operator.git
cd antigravity-operator

# Run the automated development setup (configures pre-commit hooks and builds the binary)
./scripts/setup-dev.sh
```

---

## 🧪 Testing Guidelines

Before opening a pull request, ensure all sensors pass:

```bash
# Static analysis
go vet ./...

# Unit tests with race detector
go test -v -race ./...

# Cross-compilation verification for Linux
make build-linux

# Run doctor smoke test
./bin/agyo doctor
```

---

## 📝 Commit Conventions

We strictly follow **Conventional Commits** in English:
- `feat(...)`: A new feature
- `fix(...)`: A bug fix
- `docs(...)`: Documentation changes
- `test(...)`: Adding or refactoring tests
- `refactor(...)`: Code change that neither fixes a bug nor adds a feature

Example:
```bash
git commit -m "feat(browser): support dynamic CDP port detection"
```

---

## 🚀 Submitting a Pull Request

1. Fork the repository.
2. Create your feature branch (`git checkout -b feat/my-new-feature`).
3. Commit your changes adhering to the commit guidelines.
4. Push to the branch (`git push origin feat/my-new-feature`).
5. Open a Pull Request with a clear summary of changes and test evidence. The PR template has a short checklist; for features, link the issue where the scope was agreed.

Thank you for helping empower students, researchers, and developers worldwide! 🎓
