## 🎯 Description of Changes

A clear and concise description of what changes this Pull Request introduces.

---

## 📏 Scope & Size (see [CONTRIBUTING.md](https://github.com/tiagovilasboas/antigravity-operator/blob/main/CONTRIBUTING.md#-what-gets-accepted))

- [ ] **Single subject:** this PR does one thing.
- [ ] **Linked issue:** closes/relates to #___ (required for new features and UI/API changes).
- [ ] **Size:** about 400 lines of code or less (excluding tests/generated), or the description explains why it can't be sliced.
- [ ] **Privacy:** no prompts, transcript content, or unredacted commands exposed in API/UI; nothing leaves localhost; no new reads of undocumented internal logs.

---

## 🏛️ Architectural Context (Martin Fowler Outer Harness)

- [ ] **Guia (Feedforward):** Introduces or refines prompt templates, rules, or architectural steers.
- [ ] **Sensor (Feedback):** Implements automated verification, linters, probes, or runtime tests.
- [ ] **Runtime Engine:** Modifies pure Go platform, browser supervision, or CLI engine.

---

## 🧪 Quality & Verification Checklist

- [ ] **Zero External Dependencies:** `go.mod` remains strictly on Go Standard Library (no third-party runtime dependencies).
- [ ] **Pure Go / Zero CGO:** Code builds and tests cleanly with `CGO_ENABLED=0`.
- [ ] **Concurrency Safe:** Race detector passes without warnings (`go test -race ./...`).
- [ ] **Unit Tests:** New behavior has unit test coverage (>80%).
- [ ] **Static Diagnostics:** `go vet ./...` passes and `gofmt -l .` prints nothing.
- [ ] **Cross-Platform Readiness:** Tested on or respects macOS/Linux parity.

---

## 📸 Screenshots or Terminal Evidences (if applicable)

```text
Paste your terminal output or screenshot here
```
