# Especificação 11: Student Workflow & Academic Assistant (`agyo study`)

## 1. Visão Geral

O comando `agyo study` consolida as capacidades do Google NotebookLM, Google Calendar e Google Docs em um fluxo de estudo de alta performance projetado para estudantes universitários, vestibulandos e pesquisadores. Ele implementa didática baseada na técnica Feynman com síntese estruturada de videoaulas, planejamento reverso de avaliações e checagem interativa de aprendizagem (quizzes com feedback instantâneo).

## 2. Subcomandos Canônicos

### 2.1 `agyo study ingest <url-ou-texto> --notebook <id> [--out <dir>] [--title <titulo>]`

Ingere videoaulas do YouTube ou páginas web diretamente como fontes do NotebookLM sem consumir tokens do modelo principal:

1. Registra a URL ou nota de texto via CDP (`notebook.AddSourceURL` ou `notebook.PushSource`).
2. Consulta o NotebookLM solicitando estruturação didática no método Feynman:
   - Resumo conceitual dos pontos centrais.
   - Termos-chave e definições.
   - 5 perguntas de verificação com gabarito explicativo.
3. Salva o artigo formatado em `<out>/artigo.md` (padrão: `estudos/<slug>/artigo.md`).
4. Extrai e salva cartões de estudo no formato Anki / Obsidian em `<out>/flashcards.md` com a tag `#flashcard`.

### 2.2 `agyo study exam-prep --disciplina <nome> --data <YYYY-MM-DD> [--dias <N>]`

Calcula o cronograma reverso de preparação até a data da prova:

- **Dia -D (ex: -7):** Leitura de base e fundamentos.
- **Dia -5/7:** Resolução de listas e exercícios práticos.
- **Dia -3/7:** Simulado contra o relógio e mapeamento de dúvidas.
- **Dia -1:** Revisão leve de flashcards e descanso cognitivo.
- **Dia 0:** Dia da avaliação.

**Sincronização:**
- Registra as tarefas em `.agents/session/todo.md`.
- Se o Google Workspace Bridge estiver autenticado (`agyo google auth`), cria automaticamente os blocos de estudo no Google Calendar.

### 2.3 `agyo study quiz --notebook <id> [--topic <tema>]`

Checkpoint interativo de aprendizagem no terminal:

1. Gera uma questão desafiadora de múltipla escolha baseada estritamente nas fontes do caderno.
2. Apresenta o enunciado e alternativas no terminal.
3. Aguarda a resposta do estudante via stdin.
4. Envia a resposta de volta ao NotebookLM, que atua como Tutor Feynman avaliando o raciocínio e explicando didaticamente qualquer equívoco.

## 3. Diretrizes de FinOps e Tolerância a Falhas

- **Payload Bound:** Respostas e resumos são limitados a 32KB (~8000 tokens) para prevenir estouro de contexto em sessões longas.
- **Streaming Quiescence Detection:** Consultas ao NotebookLM aguardam estabilização textual por 2 ciclos consecutivos antes de retornar a resposta final.
- **Zero Em-Dashes:** Conforme convenções de redação técnica do projeto.
