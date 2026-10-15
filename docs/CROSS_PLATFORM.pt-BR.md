# Matriz de Compatibilidade Cross-Platform (macOS & Linux)

O `antigravity-operator` (`agyo`) foi concebido para entregar a **mesma experiência de Session Agent** tanto em estações de trabalho locais (macOS) quanto em servidores, VMs ou ambientes de desenvolvimento Linux.

---

## 📊 Matriz de Ambientes

| Sistema / Ambiente | Modo Padrão | Flags do Chrome Aplicadas | Resolução do Binário do Chrome |
|---|---|---|---|
| **macOS (Darwin ARM64 / Apple Silicon)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` |
| **macOS (Darwin AMD64 / Intel)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` |
| **Linux Desktop (X11 / Wayland)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `google-chrome`, `google-chrome-stable`, `chromium` |
| **Linux Server / VPS (Sem Display)** | **Headless Automático** | `--headless=new`<br>`--disable-gpu`<br>`--disable-dev-shm-usage`<br>`--no-sandbox` | `google-chrome`, `chromium-browser` |
| **WSL2 (Windows Subsystem for Linux)** | Headless / WSLg | Se `$WAYLAND_DISPLAY` existir, usa GUI; caso contrário ativa Headless | `google-chrome` ou `/mnt/c/...` |
| **Windows AMD64 (Executável PE / Comunitário)** | Headless / GUI | Binário `.exe` estático disponibilizado nas releases; WSL2 recomendado para paridade POSIX | `chrome.exe` via caminho padrão de Program Files |
| **Docker Container** | **Headless Obrigatório** | `--headless=new`<br>`--disable-dev-shm-usage`<br>`--no-sandbox` | `chromium` instalado via apt/apk |

---

## ⚠️ Peculiaridades do Linux Resolvidas pelo `agyo`

1. **Crash de `/dev/shm` (Shared Memory):**
   * Em servidores Linux e containers Docker, o tamanho de `/dev/shm` é frequentemente restrito a 64MB. Isso causa crash aleatório do Chrome ao renderizar páginas pesadas.
   * O `agyo` injeta automaticamente `--disable-dev-shm-usage` em ambientes sem display.

2. **Detecção de Servidor Gráfico:**
   * O `agyo` inspeciona as variáveis `$DISPLAY` e `$WAYLAND_DISPLAY`.
   * Se nenhuma estiver configurada, o agente não falha tentando abrir uma janela gráfica inexistente: ele transiciona silenciosa e confiavelmente para o modo `--headless=new`.

3. **Permissões de Sandbox em Containers:**
   * Usuários sem privilégios de root em containers Linux encontram erros com o SUID sandbox do Chrome. A flag `--no-sandbox` é ativada dinamicamente nos modos headless para garantir inicialização sem fricção.

4. **Notificações Desktop (`agyo session watch`):**
   * **macOS:** Emite notificações nativas via AppleScript (`osascript`).
   * **Linux Desktop:** Emite notificações via especificação freedesktop (`notify-send` / `libnotify-bin`).
   * **Servidor Headless / Container / Fallback:** Emite o sino sonoro ANSI (`\a`) para alertar sessões tmux ou terminais sem gerar erros.
