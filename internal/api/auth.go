package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hpalmer/cookarr/internal/auth"
	"github.com/hpalmer/cookarr/internal/store"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// dummyHash is verified against when the username is wrong, so a bad username
// and a bad password take the same time.
var dummyHash, _ = auth.HashPassword("cookarr-dummy-password")

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	required := false
	if !s.cfg.AuthDisabled {
		exists, err := s.cfg.Store.OwnerExists(r.Context())
		if err != nil {
			s.internalError(w, "setup status", err)
			return
		}
		required = !exists
	}
	writeJSON(w, http.StatusOK, map[string]bool{"required": required})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthDisabled {
		writeError(w, http.StatusConflict, "auth_disabled", "authentication is disabled")
		return
	}

	var c credentials
	if !s.decodeJSON(w, r, &c) {
		return
	}

	c.Username = strings.TrimSpace(c.Username)
	if c.Username == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "username is required")
		return
	}
	if len(c.Password) < auth.MinPasswordLength {
		writeError(
			w,
			http.StatusUnprocessableEntity,
			"invalid",
			fmt.Sprintf("password must be at least %d characters", auth.MinPasswordLength),
		)
		return
	}

	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		s.internalError(w, "hash password", err)
		return
	}

	switch err := s.cfg.Store.CreateOwner(r.Context(), c.Username, hash); err {
	case nil:
	case store.ErrOwnerExists:
		writeError(w, http.StatusConflict, "already_setup", "setup has already been completed")
		return
	default:
		s.internalError(w, "create owner", err)
		return
	}
	if !s.startSession(w, r) {
		return
	}
	s.log.InfoContext(r.Context(), "owner setup completed")
	writeJSON(w, http.StatusCreated, map[string]string{"username": c.Username})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthDisabled {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ip := clientIP(r)
	if !s.limiter.allow(ip) {
		s.log.WarnContext(r.Context(), "login rate limited")
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts")
		return
	}

	var c credentials
	if !s.decodeJSON(w, r, &c) {
		return
	}

	owner, err := s.cfg.Store.Owner(r.Context())
	if err != nil && err != store.ErrNotFound {
		s.internalError(w, "load owner", err)
		return
	}

	hash := dummyHash
	if err == nil {
		hash = owner.PasswordHash
	}

	ok, _ := auth.VerifyPassword(c.Password, hash)
	if err != nil || !ok || c.Username != owner.Username {
		s.log.WarnContext(r.Context(), "login rejected")
		s.limiter.fail(ip)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	if !s.startSession(w, r) {
		return
	}

	s.log.InfoContext(r.Context(), "login succeeded")
	writeJSON(w, http.StatusOK, map[string]string{"username": owner.Username})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.cfg.Store.DeleteSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.internalError(w, "delete session", err)
			return
		}
	}
	http.SetCookie(w, s.cookie("", r, -1))
	s.log.InfoContext(r.Context(), "logout completed")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request) bool {
	token, hash, err := auth.NewSessionToken()
	if err == nil {
		err = s.cfg.Store.CreateSession(r.Context(), hash, time.Now().Add(sessionTTL))
	}
	if err != nil {
		s.internalError(w, "create session", err)
		return false
	}
	http.SetCookie(w, s.cookie(token, r, int(sessionTTL.Seconds())))
	s.log.DebugContext(r.Context(), "session created", "ttl", sessionTTL)
	return true
}

func (s *Server) cookie(value string, r *http.Request, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	}
}

// requireAuth passes through when AUTH_DISABLED is set, otherwise needs a
// valid session cookie.
func (s *Server) requireAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthDisabled {
			next(w, r)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err == nil {
			var ok bool
			ok, err = s.cfg.Store.SessionValid(r.Context(), auth.HashToken(c.Value))
			if err != nil {
				s.internalError(w, "check session", err)
				return
			}
			if ok {
				next(w, r)
				return
			}
		}
		s.log.DebugContext(r.Context(), "authentication required", "method", r.Method, "path", r.URL.Path)
		writeError(w, http.StatusUnauthorized, "unauthorized", "login required")
	})
}

func (s *Server) internalError(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiter counts failed attempts per key within a window.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *limiter) prune(key string, now time.Time) []time.Time {
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = kept
	}
	return kept
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key, time.Now())) < l.max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.prune(key, now)
	l.fails[key] = append(l.fails[key], now)
}
