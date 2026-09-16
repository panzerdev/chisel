package admin

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jpillora/sizestr"
)

// LogEntry represents a single captured log line
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// SessionInfo represents an active connection to the chisel server
type SessionInfo struct {
	ID           int32     `json:"id"`
	RemoteAddr   string    `json:"remoteAddr"`
	User         string    `json:"user"`
	ConnectedAt  time.Time `json:"connectedAt"`
	Remotes      []string  `json:"remotes"`
	BytesSent    int64     `json:"bytesSent"`
	BytesRecv    int64     `json:"bytesRecv"`
	ActiveConns  int32     `json:"activeConns"`
	CancelFn      func()       `json:"-"`
	BytesSentFn   func() int64 `json:"-"`
	BytesRecvFn   func() int64 `json:"-"`
	ActiveConnsFn func() int32 `json:"-"`
}

// TunnelInfo represents a configured or running tunnel/remote
type TunnelInfo struct {
	ID          string `json:"id"`
	User        string `json:"user,omitempty"`
	Type        string `json:"type"` // "forward", "reverse", "socks"
	Local       string `json:"local"`
	Remote      string `json:"remote"`
	Protocol    string `json:"protocol"` // "tcp", "udp"
	ActiveConns int32  `json:"activeConns"`
	BytesSent   int64  `json:"bytesSent"`
	BytesRecv   int64  `json:"bytesRecv"`
}

// ServerState provides state for server-mode telemetry
type ServerState struct {
	Sessions []*SessionInfo `json:"sessions"`
	Tunnels  []*TunnelInfo  `json:"tunnels"`
}

// ClientState provides state for client-mode telemetry
type ClientState struct {
	ServerURL       string        `json:"serverUrl"`
	ConnectionState string        `json:"connectionState"` // "disconnected", "connecting", "connected"
	ConnectedAt     *time.Time    `json:"connectedAt,omitempty"`
	Fingerprint     string        `json:"fingerprint"`
	RetryAttempt    int           `json:"retryAttempt"`
	NextRetryIn     time.Duration `json:"nextRetryIn"`
	Latency         string        `json:"latency"`
	Tunnels         []*TunnelInfo `json:"tunnels"`
}

// SystemStatus represents the high-level system telemetry
type SystemStatus struct {
	Mode            string       `json:"mode"` // "server" or "client"
	Version         string       `json:"version"`
	GoVersion       string       `json:"goVersion"`
	UptimeSeconds   int64        `json:"uptimeSeconds"`
	TotalBytesSent  int64        `json:"totalBytesSent"`
	TotalBytesRecv  int64        `json:"totalBytesRecv"`
	RateSentSec     int64        `json:"rateSentSec"`
	RateRecvSec     int64        `json:"rateRecvSec"`
	ActiveSessions  int          `json:"activeSessions"`
	ActiveTunnels   int          `json:"activeTunnels"`
	Server          *ServerState `json:"server,omitempty"`
	Client          *ClientState `json:"client,omitempty"`
}

// Hub coordinates real-time telemetry, session lists, bandwidth tracking, and logs
type Hub struct {
	mu           sync.RWMutex
	startTime    time.Time
	mode         string
	version      string
	goVersion    string

	// Counters
	totalSent atomic.Int64
	totalRecv atomic.Int64
	lastSent  atomic.Int64
	lastRecv  atomic.Int64
	rateSent  atomic.Int64
	rateRecv  atomic.Int64

	// Log buffer
	logs       []LogEntry
	maxLogs    int
	subscribers map[chan string]struct{}

	// Callbacks to query live server or client state
	serverStateFn func() *ServerState
	clientStateFn func() *ClientState
	disconnectFn  func(id int32) bool
	reconnectFn   func()
}

// NewHub creates a new Telemetry Hub
func NewHub(mode, version, goVersion string) *Hub {
	h := &Hub{
		startTime:   time.Now(),
		mode:        mode,
		version:     version,
		goVersion:   goVersion,
		maxLogs:     150,
		logs:        make([]LogEntry, 0, 150),
		subscribers: make(map[chan string]struct{}),
	}
	go h.meterLoop()
	return h
}

func (h *Hub) SetServerStateProvider(fn func() *ServerState, disconn func(id int32) bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.serverStateFn = fn
	h.disconnectFn = disconn
}

func (h *Hub) SetClientStateProvider(fn func() *ClientState, reconn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clientStateFn = fn
	h.reconnectFn = reconn
}

// AddTraffic increments total sent and received byte counters
func (h *Hub) AddTraffic(sent, recv int64) {
	if sent > 0 {
		h.totalSent.Add(sent)
	}
	if recv > 0 {
		h.totalRecv.Add(recv)
	}
}

// AddLog appends a message to the in-memory ring buffer and notifies subscribers
func (h *Hub) AddLog(msg string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Message:   msg,
	}

	h.mu.Lock()
	if len(h.logs) >= h.maxLogs {
		h.logs = h.logs[1:]
	}
	h.logs = append(h.logs, entry)

	line := fmt.Sprintf("[%s] %s", entry.Timestamp.Format("15:04:05"), msg)
	for ch := range h.subscribers {
		select {
		case ch <- line:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) GetLogs() []LogEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]LogEntry, len(h.logs))
	copy(out, h.logs)
	return out
}

func (h *Hub) SubscribeLogs() (chan string, func()) {
	h.mu.Lock()
	ch := make(chan string, 50)
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, unsubscribe
}

func (h *Hub) meterLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s := h.totalSent.Load()
		r := h.totalRecv.Load()
		ls := h.lastSent.Swap(s)
		lr := h.lastRecv.Swap(r)
		h.rateSent.Store(s - ls)
		h.rateRecv.Store(r - lr)
	}
}

// Status builds the complete system status response
func (h *Hub) Status() *SystemStatus {
	sent := h.totalSent.Load()
	recv := h.totalRecv.Load()
	rs := h.rateSent.Load()
	rr := h.rateRecv.Load()

	st := &SystemStatus{
		Mode:           h.mode,
		Version:        h.version,
		GoVersion:      h.goVersion,
		UptimeSeconds:  int64(time.Since(h.startTime).Seconds()),
		TotalBytesSent: sent,
		TotalBytesRecv: recv,
		RateSentSec:    rs,
		RateRecvSec:    rr,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.serverStateFn != nil {
		st.Server = h.serverStateFn()
		if st.Server != nil {
			st.ActiveSessions = len(st.Server.Sessions)
			st.ActiveTunnels = len(st.Server.Tunnels)
		}
	} else if h.clientStateFn != nil {
		st.Client = h.clientStateFn()
		if st.Client != nil {
			st.ActiveTunnels = len(st.Client.Tunnels)
			if st.Client.ConnectionState == "connected" {
				st.ActiveSessions = 1
			}
		}
	}

	return st
}

func (h *Hub) DisconnectSession(id int32) bool {
	h.mu.RLock()
	fn := h.disconnectFn
	h.mu.RUnlock()
	if fn != nil {
		return fn(id)
	}
	return false
}

func (h *Hub) TriggerReconnect() bool {
	h.mu.RLock()
	fn := h.reconnectFn
	h.mu.RUnlock()
	if fn != nil {
		fn()
		return true
	}
	return false
}

// FormatBytes returns human readable string for bytes
func FormatBytes(b int64) string {
	return sizestr.ToString(b)
}
