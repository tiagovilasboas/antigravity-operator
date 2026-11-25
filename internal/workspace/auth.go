package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
