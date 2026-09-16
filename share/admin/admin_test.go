package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jpillora/chisel/share/cio"
)

func TestAdminStatusAPI(t *testing.T) {
	hub := NewHub("server", "1.0.0-test", "go1.25")
	hub.SetServerStateProvider(func() *ServerState {
		return &ServerState{
			Sessions: []*SessionInfo{
				{
					ID:          1,
					RemoteAddr:  "127.0.0.1:54321",
					User:        "testuser",
					ConnectedAt: time.Now(),
					Remotes:     []string{"3000:3000"},
				},
			},
			Tunnels: []*TunnelInfo{
				{
					ID:       "sess#1-3000:3000",
					User:     "testuser",
					Type:     "forward",
					Local:    "0.0.0.0:3000",
					Remote:   "127.0.0.1:3000",
					Protocol: "tcp",
				},
			},
		}
	}, func(id int32) bool {
		return id == 1
	})

	server := NewServer(cio.NewLogger("test-admin"), hub, ServerConfig{
		Addr: "127.0.0.1:0",
	})

	mux := http.NewServeMux()
	server.registerRoutes(mux)

	// Test /api/v1/status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var status SystemStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode json: %s", err)
	}

	if status.Mode != "server" {
		t.Errorf("expected mode server, got %s", status.Mode)
	}
	if status.ActiveSessions != 1 {
		t.Errorf("expected 1 session, got %d", status.ActiveSessions)
	}
	if status.ActiveTunnels != 1 {
		t.Errorf("expected 1 tunnel, got %d", status.ActiveTunnels)
	}

	// Test /api/v1/sessions
	reqSess := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	recSess := httptest.NewRecorder()
	mux.ServeHTTP(recSess, reqSess)

	if recSess.Code != http.StatusOK {
		t.Fatalf("expected sessions 200, got %d", recSess.Code)
	}

	var sessions []*SessionInfo
	if err := json.Unmarshal(recSess.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("failed to decode sessions: %s", err)
	}
	if len(sessions) != 1 || sessions[0].User != "testuser" {
		t.Errorf("unexpected sessions content: %+v", sessions)
	}

	// Test Disconnect session 1
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/1", nil)
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d", recDel.Code)
	}

	// Test Disconnect nonexistent session 2
	reqDel404 := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/2", nil)
	recDel404 := httptest.NewRecorder()
	mux.ServeHTTP(recDel404, reqDel404)

	if recDel404.Code != http.StatusNotFound {
		t.Fatalf("expected delete status 404, got %d", recDel404.Code)
	}
}

func TestAdminAuthMiddleware(t *testing.T) {
	hub := NewHub("server", "1.0.0-test", "go1.25")
	server := NewServer(cio.NewLogger("test-admin"), hub, ServerConfig{
		Addr: "127.0.0.1:0",
		Auth: "admin:supersecret",
	})

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authorized"))
	}))

	// Without credentials
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// With correct credentials
	reqAuth := httptest.NewRequest(http.MethodGet, "/", nil)
	reqAuth.SetBasicAuth("admin", "supersecret")
	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recAuth.Code)
	}
}

func TestAdminStaticUI(t *testing.T) {
	hub := NewHub("client", "1.0.0-test", "go1.25")
	server := NewServer(cio.NewLogger("test-admin"), hub, ServerConfig{
		Addr: "127.0.0.1:0",
	})

	mux := http.NewServeMux()
	server.registerRoutes(mux)

	// Fetch index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for root UI, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Chisel Admin Console") {
		t.Errorf("expected HTML title 'Chisel Admin Console', got: %s", body)
	}

	// Fetch CSS asset
	reqCSS := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	recCSS := httptest.NewRecorder()
	mux.ServeHTTP(recCSS, reqCSS)

	if recCSS.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for app.css, got %d", recCSS.Code)
	}

	// Fetch JS asset
	reqJS := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	recJS := httptest.NewRecorder()
	mux.ServeHTTP(recJS, reqJS)

	if recJS.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for app.js, got %d", recJS.Code)
	}
}

func TestAdminServerLiveLifecycle(t *testing.T) {
	hub := NewHub("client", "1.0.0-test", "go1.25")
	server := NewServer(cio.NewLogger("test-admin"), hub, ServerConfig{
		Addr: "127.0.0.1:0",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start admin server: %s", err)
	}

	addr := server.listener.Addr().String()
	resp, err := http.Get("http://" + addr + "/api/v1/status")
	if err != nil {
		t.Fatalf("failed to query admin server: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from live server, got %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %s", err)
	}
	if !strings.Contains(string(data), `"mode":"client"`) {
		t.Fatalf("expected mode client in live response: %s", string(data))
	}
}
