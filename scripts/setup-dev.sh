#!/usr/bin/env bash
set -euo pipefail

HOOKS_ONLY=false
if [ "${1:-}" = "--hooks-only" ]; then
  HOOKS_ONLY=true
fi

install_fallback_hooks() {
  HOOKS_DIR=".git/hooks"
  if [ ! -d "$HOOKS_DIR" ]; then
    echo "⚠️ Diretório $HOOKS_DIR não encontrado; pulando instalação de hooks."
    return 0
  fi

  cat << 'EOF' > "$HOOKS_DIR/commit-msg"
#!/usr/bin/env bash
exec bash scripts/strip-agent-attribution.sh "$1"
EOF
  chmod +x "$HOOKS_DIR/commit-msg"

  cat << 'EOF' > "$HOOKS_DIR/pre-commit"
#!/usr/bin/env bash
set -e
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "These files need gofmt:"
  echo "$unformatted"
  exit 1
fi
echo "🔍 [Hook] Executando go vet..."
go vet ./...
echo "✅ Código formatado e validado com sucesso!"
EOF
  chmod +x "$HOOKS_DIR/pre-commit"

  cat << 'EOF' > "$HOOKS_DIR/pre-push"
#!/usr/bin/env bash
set -e
echo "🧪 [Hook] Executando gate completo (make check)..."
make check
EOF
  chmod +x "$HOOKS_DIR/pre-push"

  echo "✅ Git hooks fallback configurados em $HOOKS_DIR (commit-msg, pre-commit, pre-push)"
}

echo "🚀 Configurando hooks de desenvolvimento para o Antigravity Operator..."
if command -v lefthook >/dev/null 2>&1; then
  lefthook install
  echo "✅ Hooks instalados via lefthook (lefthook.yml)"
else
  install_fallback_hooks
fi

if [ "$HOOKS_ONLY" = true ]; then
  exit 0
fi

# 2. Compilar binário local
echo "🔨 Compilando binário local..."
make build

# 3. Executar o doctor
echo "🩺 Executando diagnóstico inicial:"
./bin/agyo doctor

echo ""
echo "🎉 Ambiente de desenvolvimento pronto! Para instalar a CLI globalmente, rode: make install"
