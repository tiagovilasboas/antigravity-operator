# Plano 02 - UX dos comandos: contrato da CLI

> Status: proposto · Data: 2026-10-15 · Dono: Tiago Vilas Boas
> Tasks: `.agents/session/todo.md` (prefixo `UX-`)

## Método

Mais de 20 invocações reais do binário instalado (`~/.local/bin/agyo`, `vdev`) e do binário compilado a partir do `main` (`8c8ef16`), com o Chrome isolado ligado e desligado. Exit codes medidos sem pipe.

> Nota: uma primeira medição indicou exit 0 em `browser tabs` com erro. Era falso positivo: o Chrome estava de pé (aberto pelo bug UX-1). Com o Chrome desligado, ambos saem com exit 1. Não é bug.

## Achados

### Bugs

| ID | Evidência | Impacto |
|---|---|---|
| UX-1 | `agyo notebook open --help` **abre o Chrome** (reproduzido no instalado e no source). `runNotebook` case `open`/`login` (`cmd/agyo/main.go` ~L671) não trata `--help`. | `--help` com efeito colateral. Viola o guardrail de janelas e assusta o usuário. |
| UX-2 | `agyo session --help` -> `Unknown session subcommand: --help`, exit 1. Mesmo padrão em `browser`, `hook` (default dos switches em L313, L638, L853). | `--help` funciona na raiz e em `session status`, não nos grupos. |
| UX-3 | `session status` em `~/Estudos` mostra `Objective: Não definido`, `Phase: Desconhecido` sem nenhuma dica. O parser (`internal/session/session.go` L108-126) só reconhece um formato fixo do `state.md`. | Falha silenciosa. O usuário acha que a sessão está vazia. |

### Inconsistências

| ID | Evidência |
|---|---|
| UX-4 | Três estilos de ajuda: curado (raiz, `notebooklm`), uma linha sem descrição (`Usage: agyo browser [start|status|...]`) e `flag` cru do Go (`Usage of checkpoint: -desc string`). |
| UX-5 | Idioma misturado: `Error rolling back checkpoint: nenhum checkpoint encontrado para rollback em .` |
| UX-6 | Exit codes: uso inválido sai às vezes com 1 (`agyo browser`, `session bogus`) e às vezes com 2 (`checkpoint --bogus`, via `flag.ExitOnError`). Sem contrato documentado. |
| UX-7 | `--json` parcial: existe em `doctor`, `session status`, `checkpoint --list`. `version --json` ignora a flag. Faltam `browser tabs`, `browser status`, `notebooklm list`, `notebooklm status`, `session list`. |
| UX-8 | Erros sem próximo passo: `browser tabs`/`notebooklm list` com Chrome desligado mostram `dial tcp 127.0.0.1:9222: connect: connection refused`. O `doctor` já sabe dizer "execute `agyo browser start`". |
| UX-9 | `agyo statsu` despeja o help inteiro. Sem "você quis dizer `session status`?". |
| UX-10 | `agyo rollback` sem id tenta reverter direto o último checkpoint. Comando destrutivo sem confirmação nem `--yes`. |

### O que manter

- Help raiz completo e alinhado; `notebooklm` com aliases e exemplos.
- `doctor`: ícones, remediação acionável, `--json`. É o padrão a copiar.
- `session status`: legível e com footprint em tokens.

## Contrato proposto

1. **Exit codes:** `0` sucesso · `1` erro de runtime · `2` erro de uso (flag/subcomando/argumento inválido). Documentado no README e no help raiz.
2. **`-h`/`--help`/`help`** em qualquer nível imprime a ajuda daquele nível, sai com 0 e **nunca executa ação**.
3. **Formato de erro único** (stderr):
   ```
   erro: Chrome isolado não está rodando (porta 9222)
   → agyo browser start
   ```
   Um idioma por mensagem. Decisão pendente: PT-BR ou EN (o help raiz hoje é EN).
4. **Ajuda por grupo** no formato do `notebooklm`: descrição por subcomando + 1-2 exemplos.
5. **`--json`** em todo comando de leitura; stdout só com JSON válido, logs no stderr.
6. **Destrutivo pede confirmação** em TTY; `--yes` para agentes/scripts.

## Tasks

| ID | Task | Tamanho | Aceite |
|---|---|---|---|
| UX-1 | Interceptar `--help` em `notebooklm open/login` e em todos os subcomandos com ação | P | Teste: nenhum subcomando com `--help` abre Chrome/escreve arquivo |
| UX-2 | `--help` nos grupos `session`, `browser`, `hook`, `checkpoint`, `notebooklm` | P | Teste table-driven: `agyo <grupo> --help` -> exit 0 + ajuda |
| UX-6 | Contrato de exit code (helper `usageErr` -> 2, `runtimeErr` -> 1) | P | Teste table-driven com Chrome desligado: cada comando de erro retorna o código esperado |
| UX-8 | Helper único de erro com "→ próximo passo"; mapear `connection refused :9222` -> `agyo browser start` | P | Golden test de mensagens |
| UX-5 | Padronizar idioma das mensagens de erro/usage | P | grep sem mistura em `cmd/agyo` |
| UX-4 | Ajuda curada por grupo | M | Snapshot test das ajudas |
| UX-7 | `--json` em `version`, `browser tabs/status`, `notebooklm list/status`, `session list` | M | `agyo X --json \| jq .` válido em todos |
| UX-3 | Parser do `state.md` avisa formato não reconhecido + exemplo esperado | P | Teste com o `state.md` do `~/Estudos` como fixture |
| UX-9 | Sugestão por distância de Levenshtein + erro curto | P | `agyo statsu` -> sugere `session status`, exit 2 |
| UX-10 | Confirmação em `rollback` sem id (TTY) + `--yes` | P | Teste não-TTY sem `--yes` -> exit 2 |

## Ordem sugerida

- **PR 1 (rápido, alto valor):** UX-1, UX-2, UX-6, UX-8, UX-5.
- **PR 2:** UX-4, UX-7.
- **PR 3:** UX-3, UX-9, UX-10.

Cada PR: `go test -race ./...` verde, `gofmt`, binário reinstalado e a matriz de comandos reexecutada como evidência no corpo do PR.
