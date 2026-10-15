# Plano 04: Google Workspace Bridge (Calendar & Docs)

> **Status:** Em Execução · **Dono:** Tiago Vilas Boas  
> **Prefixo de Tasks:** `WS-` · **Especificação:** [Spec 10](../spec/10-GOOGLE-WORKSPACE-BRIDGE.md)

---

## 1. Motivação & Escopo

Adicionar suporte nativo no `agyo` para conexão com o ecossistema Google Workspace (Google Calendar e Google Docs), sem dependências externas (Pure Go stdlib), fornecendo:
* Autenticação OAuth2 segura via servidor loopback local (`127.0.0.1:8085`).
* Comandos de terminal `agyo google calendar` e `agyo google docs` para o desenvolvedor.
* Ferramentas nativas no servidor MCP stdio do `agyo` para o Antigravity operar como um assistente de rotina e documentação.

---

## 2. Tarefas do Plano

| ID | Tema | Ação | Status |
|---|---|---|---|
| WS-1 | Auth & Storage | Pacote `internal/workspace`: TokenInfo, Save/Load (0600), refresh automático e loopback server. | ⏳ |
| WS-2 | Calendar Client | Métodos `ListEvents` e `CreateEvent` para Google Calendar API v3 em Go puro. | ⏳ |
| WS-3 | Docs Client | Métodos `GetDocument` (extração de texto limpo) e `CreateDocument` para Docs API v1. | ⏳ |
| WS-4 | CLI Subcommands | Integrar `agyo google` (`auth`, `status`, `calendar`, `docs`) no dispatcher da CLI. | ⏳ |
| WS-5 | MCP Tools | Adicionar ferramentas do Google Workspace ao servidor stdio MCP consumido pelo Antigravity. | ⏳ |
| WS-6 | Testes & Qualidade | Testes com `httptest.Server` para auth, calendar e docs com cobertura >80% e `-race`. | ⏳ |
| WS-7 | Docs & Sync | Atualizar índice de specs, READMEs e checklist de sessão. | ⏳ |
