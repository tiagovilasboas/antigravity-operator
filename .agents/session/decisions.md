# Decisões de Arquitetura e Sessão

## Premissas Acordadas
- **Stack:** Go 1.22+ com compilação 100% estática (`CGO_ENABLED=0`), zero dependências de runtime.
- **Princípios Canônicos:** Outer Harness (Guia × Sensor de Martin Fowler), SRP (pacotes cirúrgicos: `platform`, `session`, `profile`, `installer`, `doctor`), KISS, YAGNI, DRY, Sem Afirmação Sem Fonte.
- **Portabilidade:** Suporte nativo e transparente a macOS (Darwin arm64/amd64) e Linux (amd64/arm64) com adaptação automática a ambientes sem display (`--headless=new`, `--disable-dev-shm-usage`, `--no-sandbox`).
- **Templates Embutidos:** Uso de `//go:embed` para que o binário funcione offline e isolado (Modo Standalone) ou conectado ao `harness-core`.

## Log de Decisões
### [2026-09-28] MVP e Arquitetura Inicial
- **Contexto:** Necessidade de transformar as convenções de Session Agent do Antigravity em um produto de engenharia reproduzível e versionado.
- **Decisão:** Criado o repositório `antigravity-operator` com a CLI `agyo` contendo os subcomandos `init`, `doctor`, `browser` e `sync`.
- **Trade-offs:** Escolha de Go em vez de Python/Bash para garantir portabilidade instantânea e zero dependência de interpretadores nas máquinas de destino.

### [2026-10-08] Integração Nativa com Google NotebookLM no agyo
- **Contexto:** Necessidade de conectar os agentes do Google Antigravity e sessões locais de terminal aos cadernos e fontes do Google NotebookLM, sem depender de pacotes externos em Python ou Node, contornando a falta de API pública oficial.
- **Decisão:**
  1. Utilizar a infraestrutura já existente de Chrome isolado e CDP nativo (`internal/profile`) para orquestrar a sessão autenticada.
  2. Implementar o pacote `internal/notebook` com separação estrita de responsabilidades (SRP):
     - `session.go`: Verificação de status e autenticação da conta.
     - `client.go`: Extração e interação com cadernos (list, ask, push).
     - `mcp.go`: Servidor stdio MCP JSON-RPC 2.0 embutido no binário do `agyo`.
  3. Integrar no `agyo sync` a autoconfiguração do servidor `notebooklm` no `mcp_config.json` do Antigravity.
  4. Manter filosofia KISS/YAGNI: zero dependências de runtime de terceiros, 100% Go padrão.
- **Trade-offs:** A interação com o NotebookLM web é feita via CDP no contexto do navegador já autenticado pelo usuário, eliminando a fragilidade de RPCs obfuscados sujeitos a rotações do Google e respeitando a privacidade/isolamento do perfil.

