// Package api exposes the GPU inventory and telemetry over HTTP/JSON. It is the
// seed of the "platform" surface: a long-running process other services can
// query. Keep handlers thin and delegate to the gpu package.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/GabDevjs/hugin-telemetry/internal/buildinfo"
	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// Server serves the HTTP API backed by a gpu.Manager.
type Server struct {
	mgr *gpu.Manager
}

// NewServer builds a Server using the given manager.
func NewServer(mgr *gpu.Manager) *Server {
	return &Server{mgr: mgr}
}

// Handler returns the configured HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/version", s.handleVersion)
	mux.HandleFunc("GET /v1/devices", s.handleDevices)
	mux.HandleFunc("GET /v1/telemetry", s.handleTelemetry)
	return logging(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  buildinfo.Version,
		"commit":   buildinfo.Commit,
		"date":     buildinfo.Date,
		"backends": s.mgr.ActiveBackends(),
	})
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devs, err := s.mgr.Discover(r.Context())
	if err != nil && len(devs) == 0 {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devs})
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	devs, err := s.mgr.Discover(r.Context())
	if err != nil && len(devs) == 0 {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	want := r.URL.Query().Get("id") // optional filter by device ID

	samples := make([]gpu.Telemetry, 0, len(devs))
	for _, d := range devs {
		if want != "" && d.ID != want {
			continue
		}
		t, terr := s.mgr.Telemetry(r.Context(), d)
		if terr != nil {
			continue
		}
		samples = append(samples, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"telemetry": samples})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// logging is a tiny request logger middleware.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start // hook point: wire a real logger/metrics here
	})
}

// ListenAndServe starts the API server with sane timeouts and blocks until the
// context is canceled or the server fails.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
