# Roadmap - planos ativos

| Plano | Tema | Prefixo |
|---|---|---|
| [01](01-LIFECYCLE-HOOKS-SESSION-AGENT.md) | Lifecycle hooks: agente de sessão de verdade | `HK-` |
| [02](02-CLI-UX-CONTRATO.md) | UX e contrato da CLI | `UX-` |
| [03](03-RELEASE-SYNC-QUALIDADE.md) | Release, sync e qualidade | `G-` |

Tasks rastreadas em [`.agents/session/todo.md`](../../.agents/session/todo.md).

## Ordem consolidada

| Onda | Itens | Por quê |
|---|---|---|
| 0 | G3 | Ação local; para de carregar skill desatualizada agora |
| 1 | G2 | Evita que `agyo sync` suje o harness-core |
| 2 | UX PR 1 (UX-1, UX-2, UX-6, UX-8, UX-5) | Corrige `--help` com efeito colateral antes de publicar |
| 3 | G1 (`v0.5.0`) | Publica NotebookLM + correções acima |
| 4 | HK-0, HK-1, HK-2, HK-3 + HK-doctor | Corte mínimo do agente de sessão |
| 5 | G6, G5, UX PR 2 (UX-4, UX-7) | Higiene de supply chain, claims e JSON para agentes |
| 6 | Bugs #18/#19/#21 -> HK-6 | Checkpoint automático só com rollback seguro |
| 7 | UX PR 3, HK-4, HK-5, HK-7, G4, G7 | Refino |

Decisões pendentes: idioma das mensagens (UX-5) e dono dos hooks (HK-7).
