# Roadmap - planos ativos

| Plano | Tema | Prefixo |
|---|---|---|
| [01](01-LIFECYCLE-HOOKS-SESSION-AGENT.md) | Lifecycle hooks: agente de sessão de verdade | `HK-` |
| [02](02-CLI-UX-CONTRATO.md) | UX e contrato da CLI | `UX-` |
| [03](03-RELEASE-SYNC-QUALIDADE.md) | Release, sync e qualidade | `G-` |
| [04](04-GOOGLE-WORKSPACE-BRIDGE.md) | Google Workspace Bridge (Calendar & Docs) | `WS-` |
| [05](05-GAPS-PERFORMANCE-STUDENT-EDITION.md) | Gaps, performance e Student Edition | `GAP-`, `PERF-`, `STU-` |

Tasks rastreadas em [`.agents/session/todo.md`](../../.agents/session/todo.md).

## Ordem consolidada
 
| Onda | Itens | Status | Por quê |
|---|---|---|---|
| 0 | G3 | Concluído ✅ | Ação local; skills `.bak` limpas da raiz de skills |
| 1 | G2 | Concluído ✅ | Evita que `agyo sync` suje o harness-core através de symlinks |
| 2 | UX PR 1 (UX-1, UX-2, UX-6, UX-8, UX-5) | Concluído ✅ | Corrige `--help` com efeito colateral, códigos de saída e hints para porta 9222 |
| 3 | G1 (`v0.5.0`) | Concluído ✅ | Publica release multiplataforma `v0.5.0` e atualiza Formula do Homebrew tap |
| 4 | HK-0, HK-1, HK-2, HK-3 + HK-doctor | Concluído ✅ | Core do agente de sessão via `hooks.json` com `PreInvocation`, `PostToolUse` e `Stop` |
| 5 | G6, G5, UX PR 2 (UX-4, UX-7) | Concluído ✅ | Pinned MCPs, nota metodológica FinOps nos READMEs, ajuda curada e `--json` |
| 6 | Bugs #18/#19/#21 -> HK-6 | Concluído ✅ | Checkpoint automático seguro em comandos destrutivos |
| 7 | UX PR 3, HK-4, HK-5, HK-7, G4, G7 | Concluído ✅ | Sugestão Levenshtein, detecção de concorrência, cobertura de testes >70% e resiliência de CI |
| 8 | WS-1 a WS-7 (Plano 04) | Concluído ✅ | Google Workspace Bridge em Go puro: Calendar v3, Docs v1, OAuth2 loopback e MCP |
| 9 | GAP-01 a 05, PERF-01 a 02, STU-01 a 02 (Plano 05) | Concluído ✅ | Exporter nativo, CDP WebSocket resiliente, injeção/streaming NotebookLM, skills embutidas e agyo study |

Decisões fechadas: formato e idioma de mensagens consistentes (UX-5), coexistência idempotente e não destrutiva no `hooks.json` preservando plugins de terceiros (HK-7), isolamento de credenciais por contexto (SEC-WS-01) e proteção de caminhos e sanitização em workflows acadêmicos (SEC-STU-01 a 05).

