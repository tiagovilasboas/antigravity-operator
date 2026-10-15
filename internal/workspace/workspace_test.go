package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToken_StorageAndExpiry(t *testing.T) {
	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, "auth.json")

	// Nil token
	if err := SaveToken(authPath, nil); err == nil {
		t.Fatal("expected error saving nil token")
	}

	// Missing file load
	if _, err := LoadToken(filepath.Join(tempDir, "missing.json")); err == nil {
		t.Fatal("expected error loading missing file")
	}

	tok := &TokenInfo{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		ClientID:     "my-client-id",
		ClientSecret: "my-secret",
		Expiry:       time.Now().Add(1 * time.Hour),
		Scopes:       []string{ScopeCalendarEvents, ScopeDocuments},
	}

	if tok.IsExpired() {
		t.Fatal("token should not be expired")
	}

	expiredTok := &TokenInfo{
		AccessToken: "old",
		Expiry:      time.Now().Add(-10 * time.Minute),
	}
	if !expiredTok.IsExpired() {
		t.Fatal("expired token should report expired")
	}

	var nilTok *TokenInfo
	if !nilTok.IsExpired() {
		t.Fatal("nil token should report expired")
	}

	if err := SaveToken(authPath, tok); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	info, err := os.Stat(authPath)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	// Check 0600 on unix
	if info.Mode().Perm() != 0600 {
		t.Logf("token file mode is %o", info.Mode().Perm())
	}

	loaded, err := LoadToken(authPath)
	if err != nil {
		t.Fatalf("failed to load token: %v", err)
	}
	if loaded.AccessToken != tok.AccessToken || loaded.RefreshToken != tok.RefreshToken {
		t.Fatalf("mismatched loaded token: %+v", loaded)
	}
}

func TestAuth_BuildURLAndExchange(t *testing.T) {
	state := GenerateState()
	if len(state) != 32 {
		t.Fatalf("expected 32 hex chars state, got %d (%s)", len(state), state)
	}

	urlStr := BuildAuthURL("", "client123", "http://127.0.0.1:8085/callback", state, nil)
	if !strings.Contains(urlStr, "client_id=client123") || !strings.Contains(urlStr, "state="+state) {
		t.Fatalf("unexpected auth url: %s", urlStr)
	}

	// Mock Token Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		grantType := r.Form.Get("grant_type")
		if grantType == "authorization_code" {
			if r.Form.Get("code") != "valid-code" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"scope":         ScopeCalendarEvents + " " + ScopeDocuments,
			})
			return
		}

		if grantType == "refresh_token" {
			if r.Form.Get("refresh_token") != "mock-refresh-token" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "refreshed-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
			return
		}

		http.Error(w, "bad grant", http.StatusBadRequest)
	}))
	defer ts.Close()

	ctx := context.Background()

	// 1. Exchange Code
	tok, err := ExchangeCode(ctx, ts.Client(), ts.URL, "client123", "secret", "valid-code", "http://127.0.0.1:8085/callback")
	if err != nil {
		t.Fatalf("exchange code failed: %v", err)
	}
	if tok.AccessToken != "mock-access-token" || tok.RefreshToken != "mock-refresh-token" {
		t.Fatalf("unexpected exchanged token: %+v", tok)
	}

	// 2. Exchange with invalid code
	_, err = ExchangeCode(ctx, ts.Client(), ts.URL, "client123", "secret", "bad-code", "http://127.0.0.1:8085/callback")
	if err == nil {
		t.Fatal("expected error with bad code")
	}

	// 3. Refresh Token
	refreshed, err := RefreshAccessToken(ctx, ts.Client(), ts.URL, tok)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshed.AccessToken != "refreshed-access-token" {
		t.Fatalf("unexpected refreshed token: %s", refreshed.AccessToken)
	}

	// 4. Refresh with nil or empty refresh token
	if _, err := RefreshAccessToken(ctx, ts.Client(), ts.URL, nil); err == nil {
		t.Fatal("expected error on nil token refresh")
	}
}

func TestLoopbackServer_Lifecycle(t *testing.T) {
	state := "test-state-123"
	lb, err := StartLoopbackServer(0, state)
	if err != nil {
		t.Fatalf("failed to start loopback server: %v", err)
	}
	defer lb.Close()

	redirect := lb.RedirectURI()
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
		t.Fatalf("unexpected redirect uri: %s", redirect)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Trigger mismatch state (SEC-WS-02: must return 400 and NOT abort WaitForCode)
	resp, err := http.Get(fmt.Sprintf("%s?state=wrong-state&code=abc", redirect))
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on state mismatch, got %d", resp.StatusCode)
	}

	// 2. Trigger valid code (should be received cleanly)
	resp, err = http.Get(fmt.Sprintf("%s?state=%s&code=secret-auth-code", redirect, state))
	if err != nil {
		t.Fatalf("callback valid request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on valid code, got %d", resp.StatusCode)
	}

	code, err := lb.WaitForCode(ctx)
	if err != nil {
		t.Fatalf("unexpected error from WaitForCode: %v", err)
	}
	if code != "secret-auth-code" {
		t.Fatalf("expected code 'secret-auth-code', got %q", code)
	}
}

func TestLoopbackServer_ErrorParam(t *testing.T) {
	state := "error-state-456"
	lb, err := StartLoopbackServer(0, state)
	if err != nil {
		t.Fatalf("failed to start loopback server: %v", err)
	}
	defer lb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := http.Get(fmt.Sprintf("%s?state=%s&error=access_denied&error_description=user_declined", lb.RedirectURI(), state))
	if err != nil {
		t.Fatalf("callback error request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on error param, got %d", resp.StatusCode)
	}

	_, err = lb.WaitForCode(ctx)
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("expected access_denied error, got: %v", err)
	}
}

func TestCalendar_ListAndCreateEvents(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":          "evt-1",
						"summary":     "Sprint Planning",
						"description": "Planejamento quinzenal de tarefas",
						"location":    "Google Meet",
						"htmlLink":    "https://calendar.google.com/event?eid=123",
						"status":      "confirmed",
						"start": map[string]string{
							"dateTime": "2026-10-15T14:00:00.000Z",
						},
						"end": map[string]string{
							"dateTime": "2026-10-15T15:00:00Z",
						},
					},
					{
						"id":      "evt-2",
						"summary": "Dia Todo",
						"start": map[string]string{
							"date": "2026-10-16",
						},
						"end": map[string]string{
							"date": "2026-10-17",
						},
					},
				},
			})
			return
		}

		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events") {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			summary, _ := payload["summary"].(string)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       "evt-created-999",
				"htmlLink": "https://calendar.google.com/event?eid=999",
				"status":   "confirmed",
				"summary":  summary,
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	tokenGetter := func(ctx context.Context) (string, error) {
		return "valid-token", nil
	}

	client := NewCalendarClient(mockServer.Client(), mockServer.URL, tokenGetter)

	// 1. List Events
	events, err := client.ListEvents(ctx, "primary", time.Now(), time.Now().Add(24*time.Hour), 10)
	if err != nil {
		t.Fatalf("list events failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Summary != "Sprint Planning" || events[0].ID != "evt-1" {
		t.Fatalf("unexpected event[0]: %+v", events[0])
	}
	if events[1].Summary != "Dia Todo" {
		t.Fatalf("unexpected event[1]: %+v", events[1])
	}

	// 2. Create Event
	now := time.Now()
	created, err := client.CreateEvent(ctx, "primary", CalendarEvent{
		Summary:     "Foco Profundo em Código",
		Description: "Bloco de desenvolvimento do agyo",
		Start:       now,
		End:         now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}
	if created.ID != "evt-created-999" || created.Summary != "Foco Profundo em Código" {
		t.Fatalf("unexpected created event: %+v", created)
	}

	// 3. Validation error
	_, err = client.CreateEvent(ctx, "primary", CalendarEvent{Summary: ""})
	if err == nil {
		t.Fatal("expected error on empty summary")
	}
}

func TestDocs_GetAndCreateDocument(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-docs-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// GET /documents/{id}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/documents/") {
			id := strings.TrimPrefix(r.URL.Path, "/documents/")
			if id == "not-found" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"documentId": id,
				"title":      "RFC: Outer Harness para Antigravity",
				"revisionId": "rev-123",
				"body": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"paragraph": map[string]interface{}{
								"elements": []map[string]interface{}{
									{
										"textRun": map[string]string{
											"content": "Este é o parágrafo inicial da especificação.\n",
										},
									},
									{
										"textRun": map[string]string{
											"content": "Segunda linha com decisões de arquitetura.\n",
										},
									},
								},
							},
						},
					},
				},
			})
			return
		}

		// POST /documents
		if r.Method == http.MethodPost && r.URL.Path == "/documents" {
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"documentId": "new-doc-777",
				"title":      payload["title"],
				"revisionId": "rev-init",
			})
			return
		}

		// POST /documents/{id}:batchUpdate
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, ":batchUpdate") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"documentId": "new-doc-777",
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	tokenGetter := func(ctx context.Context) (string, error) {
		return "valid-docs-token", nil
	}

	client := NewDocsClient(mockServer.Client(), mockServer.URL, tokenGetter)

	// 1. Extract ID
	rawID := ExtractDocumentID("1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms")
	if rawID != "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms" {
		t.Fatalf("unexpected raw id: %s", rawID)
	}
	urlID := ExtractDocumentID("https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit?tab=t.0")
	if urlID != "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms" {
		t.Fatalf("unexpected url parsed id: %s", urlID)
	}
	multiAccURLID := ExtractDocumentID("https://docs.google.com/document/u/0/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit")
	if multiAccURLID != "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms" {
		t.Fatalf("unexpected multi-account url parsed id: %s", multiAccURLID)
	}

	// 2. Get Document
	doc, err := client.GetDocument(ctx, "my-doc-123")
	if err != nil {
		t.Fatalf("get document failed: %v", err)
	}
	if doc.Title != "RFC: Outer Harness para Antigravity" || doc.ID != "my-doc-123" {
		t.Fatalf("unexpected document metadata: %+v", doc)
	}
	if !strings.Contains(doc.BodyText, "Este é o parágrafo inicial") {
		t.Fatalf("expected body text, got %q", doc.BodyText)
	}

	// 3. Create Document with initial content
	newDoc, err := client.CreateDocument(ctx, "Minuta de Reunião", "Anotações iniciais")
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if newDoc.ID != "new-doc-777" || newDoc.Title != "Minuta de Reunião" {
		t.Fatalf("unexpected new document: %+v", newDoc)
	}

	// 4. Validation error on empty title
	_, err = client.CreateDocument(ctx, "", "sem titulo")
	if err == nil {
		t.Fatal("expected error on empty title")
	}
}

func TestAuth_FilePathForContext(t *testing.T) {
	geminiDir := "/tmp/.gemini"

	// Default
	p1 := AuthFilePath(geminiDir)
	if !strings.HasSuffix(p1, "google-workspace-auth.json") {
		t.Fatalf("unexpected default path: %s", p1)
	}

	p2 := AuthFilePathForContext(geminiDir, "")
	if p2 != p1 {
		t.Fatalf("empty context should match default path: %s vs %s", p2, p1)
	}

	p3 := AuthFilePathForContext(geminiDir, "default")
	if p3 != p1 {
		t.Fatalf("'default' context should match default path: %s vs %s", p3, p1)
	}

	// Trampo
	pTrampo := AuthFilePathForContext(geminiDir, "trampo")
	if !strings.HasSuffix(pTrampo, "google-workspace-auth.trampo.json") {
		t.Fatalf("unexpected trampo path: %s", pTrampo)
	}

	// Sanitization with slashes
	pSlash := AuthFilePathForContext(geminiDir, "trampo/voomp")
	if !strings.HasSuffix(pSlash, "google-workspace-auth.trampo_voomp.json") {
		t.Fatalf("unexpected sanitized path: %s", pSlash)
	}
}

func TestDocs_CreateDocument_BatchUpdateError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/documents" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"documentId": "doc-err-123",
				"title":      "Test Doc",
			})
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, ":batchUpdate") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("google docs backend failure"))
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	client := NewDocsClient(mockServer.Client(), mockServer.URL, func(ctx context.Context) (string, error) {
		return "valid-token", nil
	})

	_, err := client.CreateDocument(context.Background(), "Doc With Error", "some initial content")
	if err == nil || !strings.Contains(err.Error(), "append content api error (500)") {
		t.Fatalf("expected batchUpdate error, got: %v", err)
	}
}
