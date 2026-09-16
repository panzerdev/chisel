package admin

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jpillora/chisel/share/cio"
)

//go:embed ui/*
var uiFS embed.FS

// ServerConfig configures the admin HTTP server
type ServerConfig struct {
	Addr string
	Auth string // "user:pass"
}

// Server provides the admin HTTP web interface and REST API
type Server struct {
	*cio.Logger
	config     ServerConfig
	hub        *Hub
	httpServer *http.Server
	listener   net.Listener
}

// NewServer creates a new admin Server
func NewServer(logger *cio.Logger, hub *Hub, c ServerConfig) *Server {
	return &Server{
		Logger: logger.Fork("admin"),
		config: c,
		hub:    hub,
	}
}

// Hub returns the underlying telemetry hub
func (s *Server) Hub() *Hub {
	return s.hub
}

// Start runs the admin HTTP listener in background
func (s *Server) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("admin listen failed on %s: %w", s.config.Addr, err)
	}
	s.listener = l
	s.Infof("Dashboard available at http://%s", l.Addr().String())

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	if s.config.Auth != "" {
		handler = s.authMiddleware(handler)
	}

	s.httpServer = &http.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // allow long SSE streams
	}

	go func() {
		<-ctx.Done()
		s.httpServer.Close()
		s.listener.Close()
	}()

	go func() {
		if err := s.httpServer.Serve(l); err != nil && err != http.ErrServerClosed {
			s.Infof("HTTP server closed: %s", err)
		}
	}()

	return nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	parts := strings.SplitN(s.config.Auth, ":", 2)
	expectedUser := parts[0]
	expectedPass := ""
	if len(parts) > 1 {
		expectedPass = parts[1]
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Chisel Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// REST APIs
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", s.handleSessionDetail)
	mux.HandleFunc("/api/v1/tunnels", s.handleTunnels)
	mux.HandleFunc("/api/v1/reconnect", s.handleReconnect)
	mux.HandleFunc("/api/v1/events", s.handleEvents)

	// Embedded Static UI
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		s.Errorf("embedded fs error: %s", err)
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			// strip /assets prefix so files in ui/ are served correctly
			http.StripPrefix("/assets/", fileServer).ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := s.hub.Status()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := s.hub.Status()
	var sessions []*SessionInfo
	if status.Server != nil {
		sessions = status.Server.Sessions
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodDelete {
		if s.hub.DisconnectSession(int32(id)) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"disconnected"}`))
			return
		}
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := s.hub.Status()
	var tunnels []*TunnelInfo
	if status.Server != nil {
		tunnels = status.Server.Tunnels
	} else if status.Client != nil {
		tunnels = status.Client.Tunnels
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tunnels)
}

func (s *Server) handleReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.hub.TriggerReconnect() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"reconnecting"}`))
		return
	}
	http.Error(w, "Reconnect not supported in this mode", http.StatusBadRequest)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial logs
	for _, l := range s.hub.GetLogs() {
		fmt.Fprintf(w, "data: [%s] %s\n\n", l.Timestamp.Format("15:04:05"), l.Message)
	}
	flusher.Flush()

	logCh, unsubscribe := s.hub.SubscribeLogs()
	defer unsubscribe()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-logCh:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
