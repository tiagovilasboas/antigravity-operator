# Plano 07 - Google Calendar MCP Autonomo (Zero-Tab & MCP Nativo)

> Status: concluido · Data: 2026-10-19 · Dono: Tiago Vilas Boas
> Tasks: `.agents/session/todo.md` (prefixo `MCP-`, `QA-`)

## Contexto e Motivacao

O Google Calendar e peca-chave no workflow de estudantes, pesquisadores e desenvolvedores que utilizam coding agents. Conforme diretriz de design, **o modulo nao utiliza plugins ou abas do Chrome**: e implementado estritamente como um **servidor MCP nativo e autônomo** (`google-calendar`), operando via API REST em background com zero abas de navegador:

1. **Operacao 100% Zero-Tab via MCP:** o agente no Antigravity consome as ferramentas `google_calendar_list_events` e `google_calendar_create_event` via stdio JSON-RPC. A chamada ocorre diretamente contra a API oficial do Google Calendar em background, sem abrir navegador, sem criar abas e sem consumir memoria do Chrome.
2. **Gerenciamento Unificado de MCP:** comando `agyo google mcp [install|status|uninstall]` registra automaticamente o servidor em `default-servers.json`, `mcp_config.json` e instala os schemas JSON para as ferramentas lazy do Antigravity.
3. **Datas flexiveis e linguagem natural:** parser `ParseFlexTime` suporta RFC3339, ISO, formato brasileiro (`DD/MM/YYYY`) e expressoes relativas (`hoje 19:30`, `amanha 10:00`, `+2h`, `+30m`).
4. **Importacao direta de credenciais:** suporte a `agyo google auth --credentials <arquivo.json>` para extrair Client ID e Secret baixados do Google Cloud Console sem digitacao manual.
5. **iCalendar universal (.ics):** geracao de arquivos `.ics` padrao RFC 5545 para exportacao e compartilhamento offline seguro.

---

## Tasks Concluidas

- [x] **MCP-01:** Parser flexivel de datas e horarios (`ParseFlexTime`) em `internal/workspace/calendar.go` suportando RFC3339, ISO, formato brasileiro e expressoes relativas.
- [x] **MCP-02:** Servidor MCP dedicado `google-calendar` operando via stdio JSON-RPC (`agyo google mcp`) com zero abas de navegador e respostas em milissegundos.
- [x] **MCP-03:** Gerenciamento CLI do servidor MCP via `agyo google mcp install|status|uninstall` registrando em `default-servers.json`, `mcp_config.json` e instalando schemas JSON em `~/.gemini/antigravity/mcp/google-calendar/`.
- [x] **MCP-04:** Suporte a importacao direta de `credentials.json` do Google Cloud Console via `agyo google auth --credentials <caminho.json>`.
- [x] **MCP-05:** Geracao e exportacao de iCalendar universal (`.ics`) via `CalendarEvent.ToICS()` e flag `--ics` / subcomando `agyo google calendar ics`.
- [x] **MCP-06:** Documentacao nos READMEs (`README.md` e `README.pt-BR.md`) com paridade estrita (`TestReadmeParity`).
- [x] **QA-07:** Quality gate completo (`go test -count=1 -race ./...`, `go vet ./...`, auditoria AppSec de permissoes e zero travessoes).
