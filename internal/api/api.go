// Package api holds the HTTP handlers, middleware and routing.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store"
)

// Pager fetches pages for URL imports.
type Pager interface {
	Get(ctx context.Context, rawURL string) ([]byte, error)
}

type Config struct {
	Store        store.Store
	Fetcher      Pager
	Logger       *slog.Logger
	AuthDisabled bool
	MaxBodyBytes int64
}

type Server struct {
	cfg     Config
	log     *slog.Logger
	limiter *limiter // Limiter is a simple rate limiter
}

// New returns the API handler, rooted at /api/v1.
func New(cfg Config) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Server{cfg: cfg, log: cfg.Logger, limiter: newLimiter(5, time.Minute)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)

	mux.HandleFunc("GET /api/v1/setup", s.setupStatus)
	mux.HandleFunc("POST /api/v1/setup", s.setup)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)

	mux.Handle("POST /api/v1/recipes/import", s.requireAuth(s.importRecipe))
	mux.Handle("GET /api/v1/recipes", s.requireAuth(s.listRecipes))
	mux.Handle("GET /api/v1/recipes/{id}", s.requireAuth(s.getRecipe))
	mux.Handle("DELETE /api/v1/recipes/{id}", s.requireAuth(s.deleteRecipe))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})

	return s.logRequests(s.sameOrigin(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write JSON response", "err", err)
	}
}

// decodeJSON reads a size-limited JSON body. It writes the error response and
// returns false on failure.
func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		s.log.DebugContext(r.Context(), "request JSON rejected", "method", r.Method, "path", r.URL.Path)
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		}
		return false
	}
	return true
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		completed := false
		defer func() {
			status := sw.status
			if !completed {
				status = http.StatusInternalServerError
			}
			s.log.InfoContext(r.Context(), "request", "method", r.Method, "path", r.URL.Path, "status", status, "dur", time.Since(start), "completed", completed)
		}()
		next.ServeHTTP(sw, r)
		completed = true
	})
}

// sameOrigin rejects cross-site unsafe requests. SameSite=Lax cookies already
// keep them from being authenticated; this is a second layer.
func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					s.log.DebugContext(r.Context(), "cross-origin request rejected", "method", r.Method, "path", r.URL.Path)
					writeError(w, http.StatusForbidden, "forbidden", "cross-origin request rejected")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func hasRecipeErr(err error) bool { return errors.Is(err, capture.ErrNoRecipe) }
