package settings

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

//go:embed web/index.html
var indexHTML string

// Status is the live status shown on the settings page.
type Status struct {
	HubState  string `json:"hub_state"`
	Protected bool   `json:"protected"`
	Version   string `json:"version"`
}

// Options wires the server to the rest of the app via callbacks so this
// package stays decoupled from hubmanager/startup/updater.
type Options struct {
	// Path is the settings.json location.
	Path string
	// Apply is called after a settings change is validated and saved, letting
	// the app react (restart the Hub, toggle login registration, …).
	Apply func(Settings) error
	// OpenConfig opens the Hub's config.yaml in the default editor.
	OpenConfig func() error
	// Status returns the current live status for display (may be nil).
	Status func() Status
}

// Server is a lazily-started loopback HTTP server that serves the settings UI.
// It binds 127.0.0.1 on an OS-assigned port and gates every request on a
// per-launch random token.
type Server struct {
	opts  Options
	mu    sync.Mutex
	srv   *http.Server
	ln    net.Listener
	token string
}

// NewServer creates a settings server. Call EnsureRunning to start it.
func NewServer(opts Options) *Server {
	return &Server{opts: opts}
}

// EnsureRunning starts the server if it isn't already and returns the URL
// (including the access token) to open in a browser.
func (s *Server) EnsureRunning() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return s.urlLocked(), nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	tok, err := randToken()
	if err != nil {
		_ = ln.Close()
		return "", err
	}
	s.ln = ln
	s.token = tok

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/open-config", s.handleOpenConfig)
	mux.HandleFunc("/api/status", s.handleStatus)

	s.srv = &http.Server{Handler: s.auth(mux)}
	go func() { _ = s.srv.Serve(ln) }()
	return s.urlLocked(), nil
}

// Close shuts the server down.
func (s *Server) Close() error {
	s.mu.Lock()
	srv := s.srv
	s.srv = nil
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Close()
}

func (s *Server) urlLocked() string {
	return fmt.Sprintf("http://%s/?t=%s", s.ln.Addr().String(), s.token)
}

// auth restricts to loopback callers carrying the correct token (query param
// "t" for the page load, header "X-Scrub-Token" for API calls).
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		tok := r.Header.Get("X-Scrub-Token")
		if tok == "" {
			tok = r.URL.Query().Get("t")
		}
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page := strings.Replace(indexHTML, "__TOKEN__", s.token, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, page)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cur, err := Load(s.opts.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cur)
	case http.MethodPost:
		var in Settings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := in.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := Save(s.opts.Path, in); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if s.opts.Apply != nil {
			if err := s.opts.Apply(in); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOpenConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.opts.OpenConfig != nil {
		if err := s.opts.OpenConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var info Status
	if s.opts.Status != nil {
		info = s.opts.Status()
	}
	writeJSON(w, info)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func randToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
