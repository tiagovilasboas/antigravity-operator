# Cross-Platform Compatibility Matrix (macOS & Linux)

[🇧🇷 Leia em Português](CROSS_PLATFORM.pt-BR.md)

`antigravity-operator` (`agyo`) is designed to deliver **identical Session Agent capabilities** across local developer laptops (macOS) and remote server/container environments (Linux).

---

## 📊 Environment Support Matrix

| OS / Runtime Environment | Default Mode | Applied Chrome Flags | Chrome Binary Resolution |
|---|---|---|---|
| **macOS (Darwin ARM64 / Apple Silicon)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` |
| **macOS (Darwin AMD64 / Intel)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` |
| **Linux Desktop (X11 / Wayland)** | Desktop GUI | `--remote-debugging-port=9222`<br>`--user-data-dir=...` | `google-chrome`, `google-chrome-stable`, `chromium` |
| **Linux Server / VPS (Headless)** | **Auto-Headless** | `--headless=new`<br>`--disable-gpu`<br>`--disable-dev-shm-usage`<br>`--no-sandbox` | `google-chrome`, `chromium-browser` |
| **WSL2 (Windows Subsystem for Linux)** | Headless / WSLg | Uses GUI if `$WAYLAND_DISPLAY` is active; falls back to Headless otherwise | `google-chrome` or Windows host binary |
| **Windows AMD64 (Native PE / Community)** | Headless / GUI | Native `.exe` binary available in GitHub Releases; WSL2 recommended for POSIX parity | `chrome.exe` via standard Program Files path |
| **Docker Container** | **Mandatory Headless** | `--headless=new`<br>`--disable-dev-shm-usage`<br>`--no-sandbox` | `chromium` installed via apt/apk |

---

## ⚠️ Linux Edge Cases Handled Automatically by `agyo`

1. **Shared Memory Exhaustion (`/dev/shm` Crash):**
   * Docker containers and Linux virtual machines typically restrict `/dev/shm` to 64MB. This causes spontaneous Chrome browser crashes during heavy DOM rendering.
   * `agyo` automatically injects `--disable-dev-shm-usage` whenever running in headless server environments.

2. **Display Server Auto-Detection:**
   * `agyo` probes `$DISPLAY` and `$WAYLAND_DISPLAY`.
   * When neither is available, the agent does not crash attempting to open a window; it transitions deterministically into `--headless=new`.

3. **Container Sandbox Permissions:**
   * Non-root users in containerized Linux environments often encounter SUID sandbox failures. The `--no-sandbox` flag is applied dynamically in headless/container contexts to guarantee zero-friction startup.

4. **Desktop Notifications (`agyo session watch`):**
   * **macOS:** Dispatches native UserNotifications via AppleScript (`osascript`).
   * **Linux Desktop:** Emits freedesktop notifications via `notify-send` (`libnotify-bin`).
   * **Headless / Container / Fallback:** Emits ANSI terminal bell (`\a`) to wake up tmux or terminal sessions without crashing.
