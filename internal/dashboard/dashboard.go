package dashboard

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tiagoboas/antigravity-operator/internal/doctor"
	"github.com/tiagoboas/antigravity-operator/internal/platform"
	"github.com/tiagoboas/antigravity-operator/internal/profile"
	"github.com/tiagoboas/antigravity-operator/internal/session"
	"github.com/tiagoboas/antigravity-operator/internal/watcher"
)

//go:embed dashboard.html
var dashboardHTML string

// Config configura o servidor do dashboard local.
type Config struct {
	Port         int
	TargetDir    string
	PlatformInfo *platform.Info
	OpenBrowser  bool
}

// Server encapsula o servidor HTTP do dashboard.
type Server struct {
	cfg      Config
	server   *http.Server
	listener net.Listener
	port     int // port actually bound, used by the Host/Origin allowlist
}

// ConsolidatedData agrupa todos os dados para consumo da UI em 1 requisição.
type ConsolidatedData struct {
	Timestamp time.Time              `json:"timestamp"`
	Session   map[string]interface{} `json:"session"`
	Doctor    map[string]interface{} `json:"doctor"`
	Tabs      []profile.Tab          `json:"tabs"`
	Events    []string               `json:"events"`
}

// NewServer inicializa o servidor de dashboard com suas rotas.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.TargetDir == "" {
		cfg.TargetDir = "."
	}
	if cfg.PlatformInfo == nil {
		info, err := platform.Detect()
		if err != nil {
			info = &platform.Info{OS: runtime.GOOS, Arch: runtime.GOARCH}
		}
		cfg.PlatformInfo = info
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir porta %d para o dashboard: %w", cfg.Port, err)
	}

	mux := http.NewServeMux()
	s := &Server{
		cfg:      cfg,
		listener: listener,
		port:     listener.Addr().(*net.TCPAddr).Port,
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/all", s.handleAll)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/doctor", s.handleDoctor)
	mux.HandleFunc("/api/tabs", s.handleTabs)
	mux.HandleFunc("/api/events", s.handleEvents)

	s.server = &http.Server{
		Handler:      s.localOnly(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s, nil
}

// localOnly rejects requests whose Host or Origin is not this loopback server.
// Binding to 127.0.0.1 is not enough: with DNS rebinding a remote page can
// reach the port under its own hostname and read the JSON APIs.
func (s *Server) localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isLocalHost(r.Host) {
			http.Error(w, "forbidden: non-local Host header", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || !s.isLocalHost(u.Host) {
				http.Error(w, "forbidden: non-local Origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isLocalHost reports whether hostport names this server on loopback:
// localhost, 127.0.0.1 or [::1], on the bound port.
func (s *Server) isLocalHost(hostport string) bool {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		// No port: the client used the scheme default (80 for http).
		host, port = strings.Trim(hostport, "[]"), "80"
	}
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return port == strconv.Itoa(s.port)
	default:
		return false
	}
}

// Addr retorna o endereço TCP resolvido do listener.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// Start inicia a escuta HTTP bloqueante.
func (s *Server) Start() error {
	url := fmt.Sprintf("http://%s", s.Addr())
	fmt.Printf("🚀 Antigravity Operator Dashboard ativo em: %s\n", url)
	fmt.Println("📊 Pressione Ctrl+C para encerrar o servidor.")

	if s.cfg.OpenBrowser {
		go OpenBrowser(url)
	}

	err := s.server.Serve(s.listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown desliga o servidor graciosamente.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// OpenBrowser tenta abrir a URL no navegador padrão do sistema operacional.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

func (s *Server) handleAll(w http.ResponseWriter, r *http.Request) {
	data := ConsolidatedData{
		Timestamp: time.Now(),
		Session:   s.getSessionData(),
		Doctor:    s.getDoctorData(),
		Tabs:      s.getTabsData(),
		Events:    s.getEventsData(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.getSessionData())
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.getDoctorData())
}

func (s *Server) handleTabs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.getTabsData())
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.getEventsData())
}

func (s *Server) getSessionData() map[string]interface{} {
	resp := map[string]interface{}{
		"active": false,
	}

	sum, err := session.GetSummary(s.cfg.TargetDir)
	if err != nil {
		resp["error"] = err.Error()
		return resp
	}

	pct := 0
	if sum.TotalTasks > 0 {
		pct = (sum.DoneTasks * 100) / sum.TotalTasks
	}

	resp["active"] = true
	resp["objective"] = sum.Objective
	resp["status"] = sum.Status
	resp["doneTasks"] = sum.DoneTasks
	resp["totalTasks"] = sum.TotalTasks
	resp["progress"] = pct
	resp["pendingList"] = sum.Pending
	return resp
}

func (s *Server) getDoctorData() map[string]interface{} {
	rep := doctor.Run(s.cfg.PlatformInfo)
	checks := make([]map[string]interface{}, 0, len(rep.Checks))
	for _, c := range rep.Checks {
		msg := c.Details
		if c.PublicDetails != "" {
			msg = c.PublicDetails
		}
		checks = append(checks, map[string]interface{}{
			"name":    c.Name,
			"status":  c.Status,
			"message": msg,
		})
	}
	return map[string]interface{}{
		"os":         s.cfg.PlatformInfo.OS,
		"arch":       s.cfg.PlatformInfo.Arch,
		"hasDisplay": s.cfg.PlatformInfo.HasDisplay,
		"checks":     checks,
	}
}

func (s *Server) getTabsData() []profile.Tab {
	tabs, err := profile.ListTabs(profile.DefaultDebugPort)
	if err != nil {
		return []profile.Tab{}
	}
	return tabs
}

func (s *Server) getEventsData() []string {
	var events []string
	home, err := os.UserHomeDir()
	if err != nil {
		return events
	}
	tInfo, err := watcher.FindLatestTranscript(home)
	if err != nil {
		return events
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	opts := watcher.WatchOptions{
		Follow:       false,
		InitialSteps: 8,
		OSName:       s.cfg.PlatformInfo.OS,
	}

	_ = watcher.Stream(ctx, tInfo.Path, opts, func(evt *watcher.Event) {
		sm := evt.MetadataSummary()
		if sm != "" {
			events = append(events, sm)
		}
	})

	return events
}
