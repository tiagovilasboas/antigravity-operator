---
name: video-to-article
description: Transforma vídeos do YouTube, palestras, conferências técnicas e documentações densas em artigos de estudo estruturados, spikes de arquitetura e provas de conceito (POCs) executáveis utilizando o Google NotebookLM e o agyo. Previne context bloat externalizando transcrições e economizando ~99% de tokens.
inclusion: manual
evidence_level: curated
last_verified: 2026-10-08
---

# Video to Article — Síntese com NotebookLM & Antigravity

Esta skill estabelece a ponte operacional entre o **Google NotebookLM (Research Brain)** e o **Google Antigravity (Execution Hands)**, transformando vídeos longos do YouTube (palestras, conferências GOTO/Strange Loop/re:Invent, cursos e tutoriais) em três tipos de artefatos de alto valor para o repositório local (`~/Estudos`):

1. **Artigos de Estudo & Guias Feynman (`/estudo`):** Didática exemplar, modelo mental, analogia do cotidiano, diagramas ASCII e testes de fixação.
2. **Spikes de Arquitetura & Investigação Técnica (`/spike`):** Trade-offs, limites de escala, gotchas de produção e Architecture Decision Records (ADRs).
3. **Provas de Conceito & Protótipos Executáveis (`/poc`):** Código funcional mínimo (KISS/YAGNI), runbook de execução e testes unitários validados no terminal.

---

## 1. A Alavanca de FinOps & Context Bloat

Injetar uma transcrição bruta de vídeo de 1 hora (~25.000 tokens) ou livros técnicos inteiros (~100.000 tokens) diretamente na janela de contexto do agente causa:
- **Explosão de Custo/Tokens:** 100k tokens multiplicados por 15 turnos de sessão somam **1.500.000 de tokens de input**, estourando limites de TPM.
- **Degradação de Atenção (*Lost in the Middle*):** O agente alucina ou ignora diretrizes anteriores com tanto ruído textual.

**A Solução com NotebookLM (`agyo notebooklm`):**
O NotebookLM ingere e indexa o vídeo multimodal (áudio, transcrição e timestamps) no Gemini 1.5 Pro a **custo zero de tokens no Antigravity**. O agente apenas faz consultas cirúrgicas via CDP ou MCP (`notebooklm_ask`) de ~60 tokens e recebe resumos citados de ~500 tokens (**economia real de ~99,2% de cota**).

```text
[ Vídeo do YouTube / Conferência / Aula ]
                    │
                    ▼
       [ Google NotebookLM ] ── (Indexação multimodal, transcrição oficial e timestamps a 0 tokens)
                    │
                    ▼ (Pure-Go CDP / RFC 6455 WebSocket / stdio MCP)
       [ agyo notebooklm / MCP ] ── (`notebooklm_ask` ou `agyo notebooklm ask`)
                    │
                    ▼
       [ Antigravity / Session Agent ] ── (Estrutura artigos, spikes e POCs no repositório de estudos)
                    │
                    ▼
[ ~/Estudos/{artigos,spikes,pocs}/ ] ── (Conhecimento perene, código funcional e testes validados)
```

---

## 2. Fluxo Operacional Passo a Passo

### Passo 1: Ingestão da Fonte no NotebookLM
1. Se o Chrome isolado não estiver aberto, execute no terminal:
   ```bash
   agyo notebooklm open
   ```
2. No Chrome aberto no NotebookLM:
   - Crie ou abra um caderno temático (ex.: *"Sistemas Distribuídos & Concorrência"*).
   - Clique em **Adicionar Fonte > Link do YouTube** e cole o link do vídeo.
   - O Google processará a transcrição oficial em segundos.

### Passo 2: Descoberta e Consulta Cirúrgica
1. Liste os cadernos para obter o ID do caderno:
   ```bash
   agyo notebooklm list
   ```
2. Escolha o objetivo e consulte via terminal ou MCP (`notebooklm_ask`):

#### Para Artigo de Estudo:
```bash
agyo notebooklm ask <id> "Quais são os 3 conceitos fundamentais explicados neste vídeo? Explique cada um com o timestamp onde é apresentado, os exemplos práticos usados pelo palestrante e as armadilhas comuns para quem está aprendendo."
```

#### Para Spike de Arquitetura:
```bash
agyo notebooklm ask <id> "Qual foi o problema de engenharia ou escala que motivou essa arquitetura? Quais foram os trade-offs avaliados, tecnologias descartadas e lições aprendidas em produção relatadas na talk? Cite timestamps."
```

#### Para Prova de Conceito (POC):
```bash
agyo notebooklm ask <id> "Qual foi a implementação prática ou algoritmo demonstrado em código? Forneça a arquitetura dos componentes, interfaces, contratos de dados e o fluxo passo a passo para reproduzir esse protótipo localmente."
```

---

## 3. Os Três Modos de Entrega

### Modo 1: Artigo de Estudo (`~/Estudos/artigos/<tema>.md`)
Estrutura canônica baseada na Didática dos 80% e na Técnica Feynman:

1. **Gancho Cotidiano & Metáfora do Mundo Real:**
   - Comece sempre com uma cena física compreensível (restaurante em horário de pico, oficina mecânica, trânsito, biblioteca). O jargão técnico entra *depois*.
2. **O Ponto Cego / Problema Real:**
   - Por que o desenvolvedor comum sofre com esse tema e onde ocorrem os erros mais frequentes.
3. **Diagrama Conceitual em Bloco ASCII:**
   - Um fluxo visual claro em bloco de código `text`.
4. **Mecanismo Passo a Passo (O Deep Dive):**
   - Explicação minuciosa com exemplos em código limpo, ancorada nos timestamps do vídeo original.
5. **Resumo Executivo (Cheat Sheet):**
   - Tabela comparativa ou resumo em tópicos dos pontos essenciais para consulta rápida.
6. **Perguntas de Fixação (Comprehension Checkpoints):**
   - 3 perguntas desafiadoras com respostas ocultas ou comentadas para testar a retenção ativa.

### Modo 2: Spike de Arquitetura (`~/Estudos/spikes/<tema>-spike.md`)
Estrutura para decisões técnicas fundamentadas em engenharia de produção:

1. **Contexto & Pergunta Central:**
   - Qual hipótese está sendo investigada e qual decisão de produto/sistema depende deste spike.
2. **Arquitetura & Fluxo Proposto:**
   - Diagrama ASCII de componentes, protocolos de comunicação e fronteiras de domínio.
3. **Matriz de Trade-offs:**
   - Tabela: Abordagem vs Prós vs Contras vs Complexidade Operacional.
4. **Gotchas & Lições Aprendidas:**
   - Falhas reais reportadas no vídeo que só aparecem em produção ou sob alta carga.
5. **Veredito do Spike (ADR - Architecture Decision Record):**
   - Recomendação clara: **ADOTAR**, **REJEITAR** ou **AVALIAR MAIS**, com justificativa baseada em evidências.

### Modo 3: Prova de Conceito (`~/Estudos/pocs/<nome-da-poc>/`)
Estrutura para protótipos funcionais e código real:

1. **Scaffold Mínimo (KISS / YAGNI):**
   - Crie a pasta do projeto em `~/Estudos/pocs/<nome-da-poc>/`.
   - Implemente o código-fonte estritamente necessário para testar o conceito central.
2. **Testes Automatizados:**
   - Escreva testes unitários ou de integração que validem as asserções da talk.
3. **Validação no Terminal (Sensor Computacional):**
   - Execute a suíte de testes (`go test ./...`, `npm test`, etc.) e registre as evidências da execução.
4. **Runbook de Reprodução (`README.md` da POC):**
   - Passo a passo de 3 linhas para qualquer outro engenheiro rodar o protótipo.

---

## 4. Retroalimentação (Push Back ao Caderno)

Após gerar o artigo técnico, spike ou documentação da POC no seu repositório de estudos, envie a síntese de volta ao caderno no NotebookLM:

```bash
agyo notebooklm push <notebook-id> ~/Estudos/artigos/<tema>.md
```

**Por que fazer isso?**
O NotebookLM passa a ter como fonte tanto o vídeo original quanto o artigo/spike refinado que você produziu, permitindo que consultas futuras combinem a fala do palestrante com as suas conclusões arquiteturais consolidadas!

---

## 5. Regras Inegociáveis (Hard Constraints)

1. **Zero Em-Dashes:** Nunca use travessões longos (`—`) ou meias-riscas (`–`). Use estritamente hífens normais com espaço: ` - `.
2. **Sem Afirmações sem Fonte:** Toda citação de métrica, comportamento interno de tecnologia ou arquitetura deve conter menção ao momento do vídeo ou referência direta da fonte.
3. **Didática dos 80%:** O leitor de um artigo de estudos deve entender o "porquê" antes de ser bombardeado com especificações de API.
4. **Validação Rigorosa de POCs:** Uma POC nunca termina sem que os testes tenham sido executados no terminal com resultado positivo comprovado nos logs.
