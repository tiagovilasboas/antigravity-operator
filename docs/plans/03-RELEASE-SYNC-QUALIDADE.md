# Plano 03 - Release, sync e qualidade

> Status: proposto · Data: 2026-10-15 · Dono: Tiago Vilas Boas
> Tasks: `.agents/session/todo.md` (prefixo `G-`)

## Achados (evidência de 2026-10-15)

| ID | Risco | Evidência | Ação |
|---|---|---|---|
| G1 | 🔴 | Última release `v0.4.6` (11 dias). `git log v0.4.6..main` = 20 commits. `git tag --contains 9c0df03` (NotebookLM) vazio. `Formula/agyo.rb` aponta para `v0.4.6`. Binário local reporta `vdev` (sem `-ldflags`). | Cortar `v0.5.0`, atualizar Formula/tap, `make install` local injetando `main.Version`. |
| G2 | 🔴 | `Sync` grava em `~/.gemini/config/skills/agyo/SKILL.md` (`internal/installer/installer.go` L89), que é symlink para `~/Github/harness-core/skills/agyo`. `os.WriteFile` segue symlink e os arquivos diferem (`diff -q`). Também sobrescreve `rules/session-agent.md` sem backup (L43). | Detectar symlink e não escrever (avisar); backup datado antes de sobrescrever, igual `--update-mcp`. Teste com symlink em `t.TempDir()`. |
| G3 | 🔴 | A sessão do Antigravity carrega `~/.gemini/config/skills/agyo.bak/SKILL.md` (desatualizada, sem NotebookLM), diferente da canônica. | Mover `agyo.bak` para fora da raiz de skills. `doctor` detecta skill duplicada/`.bak` na raiz. |
| G4 | 🟡 | Cobertura: `internal/notebook` 31% (scraping DOM do NotebookLM), `cmd/agyo` 49%. Demais pacotes > 80%. | Fixtures HTML para o parser; smoke opt-in `AGYO_E2E=1` (`status`, `list`, `ask`); testes de CLI cobrindo o contrato do plano 02. |
| G5 | 🟡 | README afirma "~99% token savings" sem medição no repo. | Medir em 3 vídeos (tokens da transcrição bruta vs resposta do `ask`) e publicar método, ou reescrever como afirmação qualitativa. |
| G6 | 🟡 | `doctor`: 2 pacotes `npx` sem versão fixa (`chrome-devtools`, `playwright`). | Fixar versões em `templates/mcps/default-servers.json`; `doctor` continua alertando manifests antigos. |
| G7 | 🟢 | CI só roda `ubuntu-latest` e `macos-latest`; existe `docs/CROSS_PLATFORM.md`. | Adicionar `windows-latest` à matriz ou marcar Windows como "não testado" na doc. |

Verificados sem problema: dashboard escuta só em `127.0.0.1`; `go vet` limpo; CI com `-race`; sem dados pessoais em `.agents/` versionado.

## Ordem sugerida

1. **G3** (ação local, 1 min) e **G2** (PR pequeno com teste) - afetam o ambiente hoje.
2. **G1** - depois do PR 1 do plano 02 (para a `v0.5.0` já sair sem o bug UX-1).
3. **G6**, **G5**.
4. **G4** junto com os PRs dos planos 01 e 02.
5. **G7**.

## Critérios de aceite

- G1: `brew install` (tap) instala a `v0.5.0` e `agyo version` mostra `v0.5.0`; `agyo notebooklm` disponível.
- G2: teste prova que `sync` não escreve através de symlink e cria backup ao sobrescrever arquivo regular; `git -C ~/Github/harness-core status` limpo após `agyo sync`.
- G3: lista de skills da sessão aponta para a skill canônica; `agyo doctor` alerta se reaparecer `.bak`.
- G4: `internal/notebook` >= 70%, `cmd/agyo` >= 70%.
- G5: README sem número sem fonte.
- G6: `agyo doctor` sem warning de MCP num `init` novo.
- G7: CI verde em Windows ou doc atualizada.
