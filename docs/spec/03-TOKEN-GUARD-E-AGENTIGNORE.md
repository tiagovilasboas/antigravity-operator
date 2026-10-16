# Domínio 03: Token Guard e Proteção Contextual (`.agentignore`)

Um dos maiores desperdícios de cota de API e causas de alucinação em agentes autônomos ocorre quando o modelo carrega inadvertidamente árvores inteiras de dependências, arquivos binários, minificados ou dados sensíveis para o prompt.

O arquivo `.agentignore`, introduzido na versão `v0.4.3` e scaffolded nativamente pelo `agyo init`, define a fronteira canônica de exclusão contextual.

---

## 1. O Problema: "Context Poisoning" e Exaustão de Quota

Ao solicitar uma busca ampla de código (ex: `find_by_name`, `grep`, `directory_tree` ou leitura de diretórios), um agente sem guard-rails comete frequentemente os seguintes erros:

1. **Leitura de `node_modules/` ou `vendor/`:** Carrega dezenas de milhares de linhas de código de terceiros que não têm relação com a tarefa, estourando os limites de contexto do modelo em um único turno.
2. **Leitura de Lockfiles densos (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`):** Arquivos que frequentemente ultrapassam 10.000 a 50.000 linhas de hashes criptográficos.
3. **Leitura de Bundles Minificados (`*.min.js`, `*.bundle.js`):** Linhas únicas com centenas de kilobytes de caracteres ofuscados que degradam a taxa de tokenização.
4. **Vazamento Acidental de Segredos Locais (`.env`, `.env.local`):** Exposição inadvertida de chaves privadas e credenciais nos logs de transcrição da IA.

---

## 2. A Anatomia Canônica do `.agentignore`

O template embutido em `templates/session/agentignore` é estruturado em blocos funcionais baseados nas melhores práticas da indústria (Aider, Claude Code, Git e Google Engineering):

```gitignore
# ==============================================================================
# .agentignore - Token Blacklist & Context Optimization for AI Agents
# ==============================================================================

# 1. Pastas pesadas de dependências e pacotes
node_modules/
vendor/
.venv/
venv/
env/
__pycache__/
.bundle/

# 2. Build outputs, distribuições e caches compilados
dist/
build/
bin/
out/
target/
.next/
.nuxt/
.turbo/
.cache/
*.pyc
*.pyo
*.class
*.o
*.so
*.dylib
*.dll
*.exe

# 3. Metadados do Git e VCS
.git/
.svn/
.hg/

# 4. Lockfiles densos
package-lock.json
pnpm-lock.yaml
yarn.lock
composer.lock
Gemfile.lock
Cargo.lock
poetry.lock
flake.lock

# 5. Arquivos minificados e bundles
*.min.js
*.min.css
*.bundle.js
*.bundle.css
*.map

# 6. Dumps de banco de dados, datasets e mídias binárias
*.sql.gz
*.dump
*.sqlite3
*.db
*.csv
*.parquet
*.png
*.jpg
*.jpeg
*.gif
*.ico
*.pdf
*.zip
*.tar.gz

# 7. Arquivos de segredos locais e variáveis sensíveis
.env
.env.*
!.env.example
secrets.*
*.pem
*.key
*.id_rsa
```

---

## 3. Integração com o Lifecycle do Operador

1. **Scaffold Automático (`agyo init`):**
   - Ao inicializar o projeto, o arquivo `.agentignore` é criado na raiz se ainda não existir.
   - Se já existir, o operador preserva as customizações do desenvolvedor (`Skipped: .agentignore`).
2. **Autorrecuperação (`agyo doctor --fix`):**
   - Se um usuário ou script deletar acidentalmente o `.agentignore`, o comando `agyo doctor --fix` detecta a ausência e recria o template original imediatamente.
3. **Consumo por Ferramentas e Hooks:**
   - Ferramentas de busca de arquivos locais e MCPs utilizam o `.agentignore` como filtro de primeira classe para podar a árvore de diretórios antes de formatar os tokens do prompt.

---

## 4. Decisões Técnicas & Trade-Offs

| Decisão | O Que Foi Escolhido | Alternativa Rejeitada | Racional & Trade-Off |
|---|---|---|---|
| **Formato de Configuração** | **Arquivo `.agentignore` (padrão gitignore/glob)** | Regras hardcoded no código Go ou prompts gigantes de instrução | **Vantagem:** Desenvolvedores já dominam a sintaxe do `.gitignore`. Customizável por repositório sem recompilar o binário.<br>**Trade-off:** Exige manutenção de um arquivo adicional na raiz do workspace. |
| **Comportamento em Inicializações Existentes** | **Non-Destructive Preserving** | Sobrescrever forçadamente no `init` | **Vantagem:** Respeita as regras de exclusão customizadas já configuradas pelo time.<br>**Trade-off:** Se o template original do `agyo` evoluir com novas regras padrão, projetos antigos não recebem as adições a menos que usem `--force`. |
| **Prevenção de Segredos** | **Exclusão de `.env*` e chaves criptográficas** | Permitir envio e contar com filtros do LLM | **Vantagem:** Defesa em profundidade; dados confidenciais nunca chegam à janela de contexto.<br>**Trade-off:** Se o agente realmente precisar editar um `.env.example`, o arquivo precisa ter exceção explícita (`!.env.example`). |
