# POC: [Nome do Protótipo / Prova de Conceito]

> **Objetivo da POC:** [O que este protótipo valida concretamente no mundo real?]  
> **Fonte Base:** [Palestra / Demo / Tutorial indexado no NotebookLM]  
> **Linguagem / Stack:** [Go / TypeScript / Python / etc.]  
> **Status:** [VALIDADO / FALHOU]

---

## 1. Escopo da POC

* **O que esta POC PROVA:**
  - [Asserção 1 a ser validada]
  - [Asserção 2 a ser validada]
* **O que esta POC NÃO cobre (YAGNI):**
  - [Autenticação de produção, persistência complexa, etc.]

---

## 2. Diagrama Mínimo de Componentes

```text
[Entrada de Teste] ──► [Componente / Handler] ──► [Saída Esperada]
                               │
                               ▼
                       [Validação de Erro]
```

---

## 3. Código-Fonte Canônico & Testes

* Arquivos criados na pasta da POC (`~/Estudos/pocs/<nome-da-poc>/`):
  - `main.go` ou entrypoint
  - `*_test.go` ou suíte de testes
  - `README.md` com instruções

---

## 4. Runbook de Execução Local

```bash
# 1. Navegar até a pasta da POC
cd ~/Estudos/pocs/[nome-da-poc]

# 2. Executar os testes automatizados
go test -v ./...

# 3. Executar o protótipo
go run .
```

---

## 5. Resultados dos Testes & Evidências

```text
=== RUN   TestConceitoCentral
--- PASS: TestConceitoCentral (0.01s)
PASS
```

* **Conclusão:** [O conceito se sustentou na prática? Sim/Não e considerações para levar para produção.]
