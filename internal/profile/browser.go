package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

const (
	DefaultDebugPort = 9222
	PIDFileName      = "chrome.pid"
)

// ChromeVersionResponse reflete a resposta do endpoint /json/version do Chrome.
type ChromeVersionResponse struct {
	Browser              string `json:"Browser"`
	ProtocolVersion      string `json:"Protocol-Version"`
	UserAgent            string `json:"User-Agent"`
	V8Version            string `json:"V8-Version"`
	WebKitVersion        string `json:"WebKit-Version"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// Status indica a situação atual do navegador isolado.
type Status struct {
	IsRunning  bool
	Port       int
	PID        int
	Version    string
	ProfileDir string
	Headless   bool
}

// CheckStatus verifica se a porta padrão está ativa e respondendo com a API DevTools.
func CheckStatus(profileDir string) Status {
	return CheckStatusOnPort(profileDir, DefaultDebugPort)
}

// CheckStatusOnPort verifica uma porta específica.
func CheckStatusOnPort(profileDir string, port int) Status {
	st := Status{
		Port:       port,
		ProfileDir: profileDir,
		PID:        ReadPID(profileDir),
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(endpoint)
	if err != nil {
		st.IsRunning = false
		return st
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var ver ChromeVersionResponse
		if err := json.NewDecoder(resp.Body).Decode(&ver); err == nil {
			st.IsRunning = true
			st.Version = ver.Browser
		}
	}

	return st
}

// StartOptions configura a inicialização do Chrome isolado.
type StartOptions struct {
	Port          int
	ForceHeadless bool
}

// Start inicializa o Chrome com o perfil isolado e flags corretas para a plataforma.
func Start(info *platform.Info, opts StartOptions) error {
	if info.ChromeBin == "" {
		return fmt.Errorf("binário do Google Chrome / Chromium não encontrado no sistema")
	}

	port := opts.Port
	if port <= 0 {
		port = DefaultDebugPort
	}

	// 1. Assegurar que o diretório de perfil existe
	if err := os.MkdirAll(info.BrowserProfile, 0700); err != nil {
		return fmt.Errorf("falha ao criar pasta de perfil isolado: %w", err)
	}

	// 2. Se já estiver rodando na porta solicitada, nada a fazer
	current := CheckStatusOnPort(info.BrowserProfile, port)
	if current.IsRunning {
		return nil // já está ativo e operando
	}

	// 3. Montar flags do Chrome
	args := []string{
		fmt.Sprintf("--user-data-dir=%s", info.BrowserProfile),
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
	}

	// Decisão de Headless: forçado por flag ou obrigatório por falta de display gráfico no Linux
	needHeadless := opts.ForceHeadless || (!info.HasDisplay && info.OS == "linux")
	if needHeadless {
		args = append(args,
			"--headless=new",
			"--disable-gpu",
			"--disable-dev-shm-usage", // Crítico para Linux / Docker / VPS
			"--no-sandbox",
		)
	}

	cmd := exec.Command(info.ChromeBin, args...)
	// Desacoplar processo para rodar em background de forma independente
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar processo do Chrome: %w", err)
	}

	// Grava o PID para gerenciamento de ciclo de vida e parada graciosa
	if cmd.Process != nil {
		_ = SavePID(info.BrowserProfile, cmd.Process.Pid)
	}

	// 4. Aguardar até 5 segundos para o endpoint responder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("tempo limite excedido aguardando Chrome responder na porta %d", port)
		default:
			if CheckStatusOnPort(info.BrowserProfile, port).IsRunning {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// Stop encerra graciosamente o processo do Chrome isolado usando o arquivo de PID.
func Stop(info *platform.Info) error {
	pid := ReadPID(info.BrowserProfile)
	pidFile := filepath.Join(info.BrowserProfile, PIDFileName)

	if pid <= 0 {
		// Se não há PID salvo mas a porta está ativa, avisa
		st := CheckStatus(info.BrowserProfile)
		if st.IsRunning {
			return fmt.Errorf("Chrome está ativo na porta %d, mas o PID não foi encontrado em %s. Encerre o processo manualmente", st.Port, pidFile)
		}
		return nil // Nada para parar
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidFile)
		return nil
	}

	// Enviar SIGTERM para finalização graciosa
	_ = process.Signal(syscall.SIGTERM)

	// Aguardar até 3 segundos para confirmar que o processo encerrou
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			_ = os.Remove(pidFile)
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Se ainda estiver vivo após o timeout, força SIGKILL
	_ = process.Signal(syscall.SIGKILL)
	_ = os.Remove(pidFile)
	return nil
}

// ReadPID lê o PID gravado no diretório de perfil.
func ReadPID(profileDir string) int {
	pidFile := filepath.Join(profileDir, PIDFileName)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// SavePID grava o PID no diretório de perfil.
func SavePID(profileDir string, pid int) error {
	pidFile := filepath.Join(profileDir, PIDFileName)
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// No Unix, Signal 0 testa existência sem enviar sinal real
	return process.Signal(syscall.Signal(0)) == nil
}
