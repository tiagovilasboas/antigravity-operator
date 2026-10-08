package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestDashboardServer_Endpoints(t *testing.T) {
	tempDir := t.TempDir()

	cfg := Config{
		Port:         0, // porta dinâmica
		TargetDir:    tempDir,
		PlatformInfo: &platform.Info{OS: "darwin", Arch: "arm64", HasDisplay: true},
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("falha ao criar servidor: %v", err)
	}

	// 1. Test handleIndex
	reqIndex := httptest.NewRequest("GET", "/", nil)
	wIndex := httptest.NewRecorder()
	server.handleIndex(wIndex, reqIndex)
	if wIndex.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/', obteve: %d", wIndex.Code)
	}
	if !strings.Contains(wIndex.Body.String(), "Antigravity Operator") {
		t.Errorf("conteúdo HTML não contém Antigravity Operator")
	}

	// 2. Test handleIndex 404 on other path
	req404 := httptest.NewRequest("GET", "/random", nil)
	w404 := httptest.NewRecorder()
	server.handleIndex(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("esperava 404 em '/random', obteve: %d", w404.Code)
	}

	// 3. Test handleAll
	reqAll := httptest.NewRequest("GET", "/api/all", nil)
	wAll := httptest.NewRecorder()
	server.handleAll(wAll, reqAll)
	if wAll.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/api/all', obteve: %d", wAll.Code)
	}
	if !strings.Contains(wAll.Body.String(), "session") || !strings.Contains(wAll.Body.String(), "doctor") {
		t.Errorf("esperava chaves session e doctor no JSON /api/all, obteve: %s", wAll.Body.String())
	}

	// 4. Test handleStatus
	reqStatus := httptest.NewRequest("GET", "/api/status", nil)
	wStatus := httptest.NewRecorder()
	server.handleStatus(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/api/status', obteve: %d", wStatus.Code)
	}

	// 5. Test handleDoctor
	reqDoc := httptest.NewRequest("GET", "/api/doctor", nil)
	wDoc := httptest.NewRecorder()
	server.handleDoctor(wDoc, reqDoc)
	if wDoc.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/api/doctor', obteve: %d", wDoc.Code)
	}

	// 6. Test handleTabs
	reqTabs := httptest.NewRequest("GET", "/api/tabs", nil)
	wTabs := httptest.NewRecorder()
	server.handleTabs(wTabs, reqTabs)
	if wTabs.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/api/tabs', obteve: %d", wTabs.Code)
	}

	// 7. Test handleEvents
	reqEvents := httptest.NewRequest("GET", "/api/events", nil)
	wEvents := httptest.NewRecorder()
	server.handleEvents(wEvents, reqEvents)
	if wEvents.Code != http.StatusOK {
		t.Errorf("esperava 200 em '/api/events', obteve: %d", wEvents.Code)
	}
}

func TestDashboardServer_Lifecycle(t *testing.T) {
	cfg := Config{
		Port:         28999,
		TargetDir:    t.TempDir(),
		PlatformInfo: &platform.Info{OS: "darwin", Arch: "arm64", HasDisplay: false},
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("falha ao instanciar servidor: %v", err)
	}

	if srv.Addr() == "" {
		t.Errorf("esperava endereço de listener não vazio")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("falha ao desligar servidor: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start retornou erro inesperado: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("timeout esperando encerramento do Start")
	}
}
