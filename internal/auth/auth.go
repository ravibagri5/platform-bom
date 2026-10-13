// Package auth protects the web UI and API. It supports no authentication,
// a user header set by an authenticating proxy, and OpenID Connect sign-in
// with sessions in encrypted cookies, so no database is needed.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Modes.
const (
	ModeNone   = "none"
	ModeHeader = "header"
	ModeOIDC   = "oidc"
)

// CallbackPath is where the identity provider returns after sign-in.
const CallbackPath = "/auth/callback"

// Config selects and configures the authentication mode.
type Config struct {
	Mode string

	// Header mode.
	Header         string
	TrustedProxies []netip.Prefix

	// OIDC mode.
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	UsernameClaim string
	GroupsClaim   string
	AllowedGroups []string

	// SessionSecret encrypts session cookies; empty generates one per start.
	SessionSecret   []byte
	SessionDuration time.Duration
}

// User is the signed-in user.
type User struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type userKey struct{}

// UserFrom returns the user signed in for the request, if any.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok
}

// New returns middleware that authenticates requests to next.
func New(cfg Config) (func(http.Handler) http.Handler, error) {
	switch cfg.Mode {
	case "", ModeNone:
		return func(next http.Handler) http.Handler {
			return withMe(next, ModeNone, false, func(*http.Request) (User, bool) { return User{}, false })
		}, nil
	case ModeHeader:
		return headerAuth(cfg)
	case ModeOIDC:
		return newOIDC(cfg)
	}
	return nil, fmt.Errorf("unknown auth mode %q: use none, header or oidc", cfg.Mode)
}

func headerAuth(cfg Config) (func(http.Handler) http.Handler, error) {
	if cfg.Header == "" {
		return nil, errors.New("header mode needs a header name")
	}
	if len(cfg.TrustedProxies) == 0 {
		slog.Warn("auth header is accepted from any client; set trusted proxies or restrict network access to the proxy", "header", cfg.Header)
	}
	user := func(r *http.Request) (User, bool) {
		name := strings.TrimSpace(r.Header.Get(cfg.Header))
		return User{Name: name}, name != ""
	}
	return func(next http.Handler) http.Handler {
		inner := withMe(next, ModeHeader, false, user)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if public(r) {
				next.ServeHTTP(w, r)
				return
			}
			if !trusted(r, cfg.TrustedProxies) {
				writeError(w, http.StatusForbidden, "requests must come through the authenticating proxy", "")
				return
			}
			u, ok := user(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing "+cfg.Header+" header from the authenticating proxy", "")
				return
			}
			inner.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
		})
	}, nil
}

func trusted(r *http.Request, proxies []netip.Prefix) bool {
	if len(proxies) == 0 {
		return true
	}
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	addr := ap.Addr().Unmap()
	for _, p := range proxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// public paths need no sign-in.
func public(r *http.Request) bool {
	return r.URL.Path == "/healthz"
}

type meResponse struct {
	Mode   string `json:"mode"`
	User   *User  `json:"user,omitempty"`
	Logout bool   `json:"logout,omitempty"`
}

// withMe serves GET /api/me and passes everything else to next.
func withMe(next http.Handler, mode string, logout bool, user func(*http.Request) (User, bool)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me" {
			next.ServeHTTP(w, r)
			return
		}
		resp := meResponse{Mode: mode, Logout: logout}
		if u, ok := UserFrom(r.Context()); ok {
			resp.User = &u
		} else if u, ok := user(r); ok {
			resp.User = &u
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func writeError(w http.ResponseWriter, code int, msg, login string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	body := map[string]string{"error": msg}
	if login != "" {
		body["login"] = login
	}
	_ = json.NewEncoder(w).Encode(body)
}

// safeRedirect keeps post-login redirects on this site.
func safeRedirect(rd string) string {
	if !strings.HasPrefix(rd, "/") || strings.HasPrefix(rd, "//") || strings.ContainsAny(rd, "\\\r\n") {
		return "/"
	}
	if u, err := url.Parse(rd); err != nil || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	return rd
}

func randomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
