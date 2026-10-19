package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAuthFilename = "google-workspace-auth.json"
	DefaultOAuthPort    = 8085
	GoogleAuthEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenEndpoint = "https://oauth2.googleapis.com/token"

	ScopeCalendarEvents = "https://www.googleapis.com/auth/calendar.events"
	ScopeDocuments      = "https://www.googleapis.com/auth/documents"
	ScopeDriveFile      = "https://www.googleapis.com/auth/drive.file"
)

// TokenInfo holds OAuth2 credentials for Google Workspace APIs.
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry"`
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	Context      string    `json:"context,omitempty"`
}

// IsExpired reports whether the access token has expired (with a 30s buffer).
func (t *TokenInfo) IsExpired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(30 * time.Second).After(t.Expiry)
}

// AuthFilePath returns the canonical credential file path inside the gemini directory.
func AuthFilePath(geminiDir string) string {
	return AuthFilePathForContext(geminiDir, "")
}

// AuthFilePathForContext returns context-isolated credential file paths (e.g. trampo vs pessoal).
func AuthFilePathForContext(geminiDir, contextName string) string {
	contextName = strings.TrimSpace(contextName)
	if contextName == "" || contextName == "default" {
		return filepath.Join(geminiDir, DefaultAuthFilename)
	}
	safe := strings.ToLower(contextName)
	safe = strings.ReplaceAll(safe, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	return filepath.Join(geminiDir, fmt.Sprintf("google-workspace-auth.%s.json", safe))
}

// MaxCredentialsFileSize limits credentials.json import to 1 MB to prevent resource exhaustion attacks (PLUG-04).
const MaxCredentialsFileSize = 1024 * 1024 // 1 MB

// OAuthClientCredentials stores OAuth2 client credentials imported defensivelmente from Google Cloud Console.
// ClientSecret is tagged with json:"-" to prevent secret leakage in any JSON serialization (PLUG-04).
type OAuthClientCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"-"`
	ProjectID    string `json:"project_id,omitempty"`
}

// MaskSecret returns a masked representation of a secret, revealing at most 3 characters at start and end.
func MaskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:3] + "..." + secret[len(secret)-3:]
}

// String redacts the client secret to prevent accidental leakage in logs.
func (c *OAuthClientCredentials) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("OAuthClientCredentials(ClientID=%s, ClientSecret=[REDACTED], ProjectID=%s)", c.ClientID, c.ProjectID)
}

// SafeSummary returns non-sensitive metadata safe for terminal or JSON output.
func (c *OAuthClientCredentials) SafeSummary() map[string]interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"client_id":  c.ClientID,
		"has_secret": c.ClientSecret != "",
		"project_id": c.ProjectID,
	}
}

// ParseGoogleCredentials parses a Google Cloud Console credentials.json file defensivelmente,
// enforcing size limits (<= 1 MB), file type checks, path traversal prevention,
// and secret redaction in error messages (PLUG-04).
func ParseGoogleCredentials(path string) (*OAuthClientCredentials, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("caminho do arquivo de credenciais não pode ser vazio")
	}
	if strings.ContainsRune(path, 0) {
		return nil, errors.New("caminho de arquivo inválido: caractere nulo detectado")
	}

	cleanPath := filepath.Clean(path)
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".json" {
		return nil, fmt.Errorf("arquivo de credenciais inválido: deve possuir extensão .json (recebido: %q)", ext)
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("acesso ao arquivo de credenciais: %w", err)
	}

	// Resolve symlink target if applicable
	targetPath := cleanPath
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("avaliar link de credenciais: %w", err)
		}
		targetPath = resolved
		info, err = os.Stat(targetPath)
		if err != nil {
			return nil, fmt.Errorf("acesso ao arquivo apontado pelo link de credenciais: %w", err)
		}
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("arquivo de credenciais deve ser um arquivo regular (não diretório, pipe ou dispositivo)")
	}

	if info.Size() > MaxCredentialsFileSize {
		return nil, fmt.Errorf("arquivo de credenciais excede o limite máximo permitido de 1 MB (tamanho: %d bytes)", info.Size())
	}
	if info.Size() == 0 {
		return nil, errors.New("arquivo de credenciais está vazio (0 bytes)")
	}

	f, err := os.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("abrir arquivo de credenciais: %w", err)
	}
	defer f.Close()

	// Read up to limit + 1 to detect dynamic growth or streaming sources
	data, err := io.ReadAll(io.LimitReader(f, MaxCredentialsFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("ler arquivo de credenciais: %w", err)
	}
	if int64(len(data)) > MaxCredentialsFileSize {
		return nil, errors.New("arquivo de credenciais excede o limite máximo permitido de 1 MB")
	}

	var raw struct {
		Installed *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			ProjectID    string `json:"project_id"`
		} `json:"installed"`
		Web *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			ProjectID    string `json:"project_id"`
		} `json:"web"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		ProjectID    string `json:"project_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("estrutura JSON de credenciais inválida: %w", err)
	}

	var cid, csec, pid string
	if raw.Installed != nil {
		cid = raw.Installed.ClientID
		csec = raw.Installed.ClientSecret
		pid = raw.Installed.ProjectID
	} else if raw.Web != nil {
		cid = raw.Web.ClientID
		csec = raw.Web.ClientSecret
		pid = raw.Web.ProjectID
	} else {
		cid = raw.ClientID
		csec = raw.ClientSecret
		pid = raw.ProjectID
	}

	cid = strings.TrimSpace(cid)
	csec = strings.TrimSpace(csec)
	pid = strings.TrimSpace(pid)

	if cid == "" {
		return nil, errors.New("credenciais inválidas: client_id ausente ou vazio")
	}
	if csec == "" {
		return nil, errors.New("credenciais inválidas: client_secret ausente ou vazio")
	}
	if len(cid) > 512 || len(csec) > 512 {
		return nil, errors.New("credenciais inválidas: campos excedem tamanho máximo permitido")
	}

	return &OAuthClientCredentials{
		ClientID:     cid,
		ClientSecret: csec,
		ProjectID:    pid,
	}, nil
}

// SaveToken persists the token to disk with restricted 0600 file permissions and 0700 directory permissions.
func SaveToken(path string, token *TokenInfo) error {
	if token == nil {
		return errors.New("cannot save nil token")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("set config dir permissions: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("set token file permissions: %w", err)
	}
	return nil
}

// LoadToken reads and parses the token from disk.
func LoadToken(path string) (*TokenInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token TokenInfo
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parse token json: %w", err)
	}
	return &token, nil
}

// GetValidAccessToken carrega o token para o contexto informado, atualiza-o se expirado via refresh token e retorna o access token valido.
func GetValidAccessToken(ctx context.Context, geminiDir, contextName string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if geminiDir == "" {
		home, _ := os.UserHomeDir()
		geminiDir = filepath.Join(home, ".gemini", "antigravity")
	}
	authPath := AuthFilePathForContext(geminiDir, contextName)
	tok, err := LoadToken(authPath)
	if err != nil {
		return "", fmt.Errorf("Google Workspace não autenticado. Execute 'agyo google auth' no terminal para conectar sua conta: %w", err)
	}
	if tok.IsExpired() && tok.RefreshToken != "" {
		refreshed, err := RefreshAccessToken(ctx, nil, "", tok)
		if err == nil {
			_ = SaveToken(authPath, refreshed)
			return refreshed.AccessToken, nil
		}
	}
	return tok.AccessToken, nil
}

// GenerateState returns a cryptographically secure random state parameter.
func GenerateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildAuthURL constructs the Google OAuth 2.0 consent URL.
func BuildAuthURL(endpoint, clientID, redirectURI, state string, scopes []string) string {
	if endpoint == "" {
		endpoint = GoogleAuthEndpoint
	}
	if len(scopes) == 0 {
		// Principle of least privilege: default to calendar and documents only.
		scopes = []string{ScopeCalendarEvents, ScopeDocuments}
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)

	return fmt.Sprintf("%s?%s", endpoint, q.Encode())
}

// ExchangeCode exchanges an authorization code for access and refresh tokens.
func ExchangeCode(ctx context.Context, client *http.Client, endpoint, clientID, clientSecret, code, redirectURI string) (*TokenInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if endpoint == "" {
		endpoint = GoogleTokenEndpoint
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	token := &TokenInfo{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Expiry:       time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}
	if raw.Scope != "" {
		token.Scopes = strings.Split(raw.Scope, " ")
	}
	return token, nil
}

// RefreshAccessToken refreshes an expired access token using the stored refresh_token.
func RefreshAccessToken(ctx context.Context, client *http.Client, endpoint string, token *TokenInfo) (*TokenInfo, error) {
	if token == nil || token.RefreshToken == "" {
		return nil, errors.New("cannot refresh token: refresh_token is missing")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if endpoint == "" {
		endpoint = GoogleTokenEndpoint
	}

	data := url.Values{}
	data.Set("client_id", token.ClientID)
	data.Set("client_secret", token.ClientSecret)
	data.Set("refresh_token", token.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	token.AccessToken = raw.AccessToken
	if raw.TokenType != "" {
		token.TokenType = raw.TokenType
	}
	token.Expiry = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.Scope != "" {
		token.Scopes = strings.Split(raw.Scope, " ")
	}
	return token, nil
}

// LoopbackServer manages a temporary local HTTP server to receive OAuth callback codes.
type LoopbackServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	state    string
	codeChan chan string
	errChan  chan error
	once     sync.Once
}

// StartLoopbackServer starts listening on 127.0.0.1:<port>.
func StartLoopbackServer(port int, state string) (*LoopbackServer, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	lb := &LoopbackServer{
		listener: ln,
		port:     ln.Addr().(*net.TCPAddr).Port,
		state:    state,
		codeChan: make(chan string, 10),
		errChan:  make(chan error, 10),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", lb.handleCallback)

	lb.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		_ = lb.server.Serve(ln)
	}()

	return lb, nil
}

// RedirectURI returns the local loopback URL configured for OAuth.
func (lb *LoopbackServer) RedirectURI() string {
	return fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", lb.port)
}

// WaitForCode blocks until authorization code is received or context is canceled.
func (lb *LoopbackServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-lb.errChan:
		return "", err
	case code := <-lb.codeChan:
		return code, nil
	}
}

// Close gracefully terminates the loopback server.
func (lb *LoopbackServer) Close() {
	lb.once.Do(func() {
		if lb.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = lb.server.Shutdown(ctx)
		}
	})
}

func (lb *LoopbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	retState := q.Get("state")
	if retState != lb.state {
		// AppSec SEC-WS-02: Do not send to errChan on unexpected state mismatch.
		// A probe or extraneous request should not abort WaitForCode for the real callback.
		http.Error(w, "Invalid OAuth state parameter", http.StatusBadRequest)
		return
	}

	errParam := q.Get("error")
	if errParam != "" {
		desc := q.Get("error_description")
		http.Error(w, fmt.Sprintf("Google Auth Error: %s - %s", errParam, desc), http.StatusBadRequest)
		select {
		case lb.errChan <- fmt.Errorf("auth error: %s (%s)", errParam, desc):
		default:
		}
		return
	}

	code := q.Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		select {
		case lb.errChan <- errors.New("missing code in oauth callback"):
		default:
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>agyo: Google Workspace Autenticado</title></head>
<body style="font-family: system-ui, sans-serif; text-align: center; padding: 3rem; background: #0f172a; color: #f8fafc;">
  <h1 style="color: #38bdf8;">Autenticação Concluída com Sucesso!</h1>
  <p>O <strong>agyo</strong> agora está conectado ao seu Google Workspace (Calendar & Docs).</p>
  <p style="color: #94a3b8;">Você já pode fechar esta aba do navegador e retornar ao terminal.</p>
</body>
</html>`))

	select {
	case lb.codeChan <- code:
	default:
	}
}

// GoogleCredentialsJSON models credentials files downloaded from Google Cloud Console (PLUG-04).
type GoogleCredentialsJSON struct {
	Installed *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
	Web *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"web"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// ParseCredentialsFile reads a credentials.json file downloaded from Google Cloud Console,
// extracting client_id and client_secret with full defensive security controls (PLUG-04).
func ParseCredentialsFile(path string) (clientID, clientSecret string, err error) {
	creds, err := ParseGoogleCredentials(path)
	if err != nil {
		return "", "", err
	}
	return creds.ClientID, creds.ClientSecret, nil
}

// ParseCredentialsJSON parses credentials JSON bytes downloaded from Google Cloud Console (PLUG-04).
func ParseCredentialsJSON(data []byte) (clientID, clientSecret string, err error) {
	if int64(len(data)) > MaxCredentialsFileSize {
		return "", "", fmt.Errorf("arquivo de credenciais excede o limite máximo de 1 MB (tamanho: %d bytes)", len(data))
	}
	var creds GoogleCredentialsJSON
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", "", fmt.Errorf("decodificar JSON de credenciais: %w", err)
	}

	if creds.Installed != nil && creds.Installed.ClientID != "" {
		return creds.Installed.ClientID, creds.Installed.ClientSecret, nil
	}
	if creds.Web != nil && creds.Web.ClientID != "" {
		return creds.Web.ClientID, creds.Web.ClientSecret, nil
	}
	if creds.ClientID != "" {
		return creds.ClientID, creds.ClientSecret, nil
	}

	return "", "", errors.New("não foi possível encontrar client_id e client_secret válidos no JSON informado")
}
