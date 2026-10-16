# Plano 05: Correção de Gaps, Performance e Student Edition

> **Status:** Em Execução  
> **Meta:** Corrigir os gaps técnicos mapeados no code review profundo, otimizar I/O e concorrência, e integrar os recursos da Student Edition de ponta a ponta no `agyo`.  
> **Premissas:** Go puro (zero dependências externas), zero em-dashes, cobertura de testes com `-race` 100% verde, conformidade com o Outer Harness de Fowler.

---

## 1. Escopo & Divisão de Tarefas

### Fase 1: Gaps Técnicos e Robustez de Runtime
- [ ] **GAP-01 (Exporter & Métricas):** Corrigir formato de `tool_calls` em `internal/exporter/exporter.go` (`tc.Name` em vez de `tc.Function.Name`), corrigir contagem de subagentes e substituir `bufio.Scanner` limitado por leitor seguro de até 10 MB.
- [ ] **GAP-02 (CDP WebSocket Resiliente):** Ajustar `internal/profile/cdp.go` adicionando deadline de conexão (15s), leitura em loop até matching de `id: 1` e descarte de eventos assíncronos/frames de controle.
- [ ] **GAP-03 (NotebookLM Push Real):** Implementar preenchimento real de título e conteúdo de texto na interface web do NotebookLM em `internal/notebook/client.go` (`PushSource`).
- [ ] **GAP-04 (NotebookLM Multi-Turn Ask):** Corrigir `AskNotebook` em `internal/notebook/client.go` para medir `initialCount` de mensagens e aguardar estabilização do texto em streaming antes de retornar.
- [ ] **GAP-05 (Student Skills no Binário & Doctor):** Mover as skills acadêmicas (`feynman-code-tutor`, `student-study-planner`, `token-budget-guard`, `video-to-article`) para `templates/skills/`, embutir no binário e instalá-las no `agyo sync`. Adicionar check de Google Workspace no `internal/doctor/doctor.go` e reparo de hooks no `doctor --fix`.

### Fase 2: Performance e Otimização de I/O
- [ ] **PERF-01 (Dashboard & Watcher Otimizados):** Otimizar `internal/dashboard/dashboard.go` cacheando diagnósticos pesados (TTL 15s) e otimizar `internal/watcher/watcher.go` para ler os últimos bytes via seek reverso quando `Follow == false`.
- [ ] **PERF-02 (Fast-Path de Redação no Analytics):** Otimizar `internal/analytics/analytics.go` com fast-path de paths seguros em `trackFile` para evitar execução repetida de 13 regexes pesadas.

### Fase 3: Student Edition Core Feature (`agyo study`)
- [ ] **STU-01 (CLI agyo study):** Implementar subcomando `agyo study` com suporte a `ingest`, `exam-prep` e `quiz`, integrando NotebookLM, Google Calendar e Google Docs em fluxo contínuo.
- [ ] **STU-02 (Testes & CI):** Validar toda a suíte com `go test -count=1 -race ./...` e verificar 100% de aprovação.
