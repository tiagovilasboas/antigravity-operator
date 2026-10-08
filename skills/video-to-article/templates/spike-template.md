# SPIKE: [Título da Investigação Técnica]

> **Pergunta Central do Spike:** [Qual hipótese ou dúvida técnica motivou este spike?]  
> **Fonte Base:** [Palestra / Talk / Documentação indexada no NotebookLM]  
> **Data:** [Data]  
> **Status:** [EM INVESTIGAÇÃO / CONCLUÍDO]

---

## 1. Contexto e Motivação

[Por que estamos avaliando essa tecnologia ou padrão? Qual gargalo de escala, confiabilidade ou custo enfrentamos hoje?]

---

## 2. Visão Geral da Abordagem Proposta

```text
[Cliente / Ingestão] ──► [Componente em Análise] ──► [Armazenamento / Downstream]
                                   │
                                   ▼
                       [Mecanismo de Fallback]
```

[Descrição da solução demonstrada na palestra e como ela se comportaria na nossa infraestrutura.]

---

## 3. Matriz de Trade-offs

| Critério | Abordagem Atual | Abordagem Proposta (do Vídeo) | Alternativa Concorrente |
|---|---|---|---|
| **Latência (p99)** | | | |
| **Throughput / Escala** | | | |
| **Complexidade Operacional** | | | |
| **Custo de Infraestrutura / FinOps**| | | |
| **Curva de Aprendizado da Squad** | | | |

---

## 4. Gotchas & Lições Aprendidas em Produção

[Extraídos diretamente dos relatos do palestrante com timestamps no vídeo:]
- **Gotcha 1 ([Timestamp]):** [O que quebra primeiro e como mitigar]
- **Gotcha 2 ([Timestamp]):** [Limites de escala não documentados]
- **Gotcha 3 ([Timestamp]):** [Impacto em observabilidade ou recuperação de desastres]

---

## 5. Veredito Técnico & Recomendação (ADR)

* **Decisão:** **[ADOTAR / REJEITAR / POSTERGAR]**
* **Justificativa Baseada em Evidências:** [Argumento técnico resumido em 2 parágrafos]
* **Próximos Passos (se ADOTAR):**
  1. [Passo 1]
  2. [Passo 2]
