package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	sessionCookie = "pbom_session"
	flowCookie    = "pbom_signin"
	flowTTL       = 10 * time.Minute
)

type session struct {
	User User `json:"u"`
}

// flow carries the values one sign-in must return with.
type flow struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Redirect string `json:"r"`
}

type oidcAuth struct {
	cfg     Config
	cookies cookies
	baseURL string

	mu         sync.Mutex
	provider   *oidc.Provider
	endSession string
}

func newOIDC(cfg Config) (func(http.Handler) http.Handler, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.RedirectURL == "" {
		return nil, errors.New("oidc mode needs an issuer, a client ID and a redirect URL")
	}
	u, err := url.Parse(cfg.RedirectURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != CallbackPath {
		return nil, fmt.Errorf("redirect URL must be an absolute URL ending in %s, such as https://pbom.example.com%s", CallbackPath, CallbackPath)
	}
	secret := cfg.SessionSecret
	switch {
	case len(secret) == 0:
		secret = []byte(randomString())
		slog.Warn("no session secret set; users must sign in again after pbom restarts")
	case len(secret) < 32:
		return nil, errors.New("session secret must be at least 32 bytes")
	}
	if cfg.SessionDuration <= 0 {
		cfg.SessionDuration = 12 * time.Hour
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	} else if !slices.Contains(cfg.Scopes, oidc.ScopeOpenID) {
		cfg.Scopes = append([]string{oidc.ScopeOpenID}, cfg.Scopes...)
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	s, err := newSealer(secret)
	if err != nil {
		return nil, err
	}
	o := &oidcAuth{cfg: cfg, cookies: cookies{sealer: s, secure: u.Scheme == "https"}, baseURL: u.Scheme + "://" + u.Host}
	return o.middleware, nil
}

func (o *oidcAuth) middleware(next http.Handler) http.Handler {
	inner := withMe(next, ModeOIDC, true, func(*http.Request) (User, bool) { return User{}, false })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			o.login(w, r)
			return
		case CallbackPath:
			o.callback(w, r)
			return
		case "/auth/logout":
			o.logout(w, r)
			return
		}
		if public(r) {
			next.ServeHTTP(w, r)
			return
		}
		s, ok := get[session](o.cookies, r, sessionCookie)
		if !ok {
			// The UI shell holds no data and sends users to sign-in on the API's
			// 401, keeping the #fragment the server never sees.
			if !strings.HasPrefix(r.URL.Path, "/api/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "sign-in required", "/auth/login")
			return
		}
		inner.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, s.User)))
	})
}

func (o *oidcAuth) getProvider(ctx context.Context) (*oidc.Provider, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil {
		return o.provider, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	p, err := oidc.NewProvider(ctx, o.cfg.Issuer)
	if err != nil {
		return nil, err
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	_ = p.Claims(&meta)
	o.provider, o.endSession = p, meta.EndSession
	return p, nil
}

func (o *oidcAuth) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     o.cfg.ClientID,
		ClientSecret: o.cfg.ClientSecret,
		RedirectURL:  o.cfg.RedirectURL,
		Endpoint:     p.Endpoint(),
		Scopes:       o.cfg.Scopes,
	}
}

func (o *oidcAuth) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p, err := o.getProvider(r.Context())
	if err != nil {
		slog.Error("identity provider unavailable", "issuer", o.cfg.Issuer, "error", err)
		http.Error(w, "The identity provider is unavailable. Try again later.", http.StatusBadGateway)
		return
	}
	f := flow{
		State:    randomString(),
		Nonce:    randomString(),
		Verifier: oauth2.GenerateVerifier(),
		Redirect: safeRedirect(r.URL.Query().Get("rd")),
	}
	if err := o.cookies.set(w, flowCookie, "/auth/", f, flowTTL); err != nil {
		http.Error(w, "cannot start sign-in", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, o.oauth(p).AuthCodeURL(f.State, oidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier)), http.StatusFound)
}

func (o *oidcAuth) callback(w http.ResponseWriter, r *http.Request) {
	fail := func(code int, msg string, args ...any) {
		slog.Warn("sign-in failed", append([]any{"reason", msg}, args...)...)
		http.Error(w, "Sign-in failed: "+msg+". Start again at /auth/login.", code)
	}
	f, ok := get[flow](o.cookies, r, flowCookie)
	o.cookies.clear(w, flowCookie, "/auth/")
	if !ok {
		fail(http.StatusBadRequest, "the sign-in expired or was started in another browser")
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		fail(http.StatusUnauthorized, "the identity provider refused the sign-in", "error", e, "description", q.Get("error_description"))
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.State)) != 1 {
		fail(http.StatusBadRequest, "state mismatch")
		return
	}
	p, err := o.getProvider(r.Context())
	if err != nil {
		fail(http.StatusBadGateway, "the identity provider is unavailable", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	tok, err := o.oauth(p).Exchange(ctx, q.Get("code"), oauth2.VerifierOption(f.Verifier))
	if err != nil {
		fail(http.StatusBadGateway, "the authorization code could not be exchanged", "error", err)
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		fail(http.StatusBadGateway, "the identity provider returned no ID token")
		return
	}
	idt, err := p.Verifier(&oidc.Config{ClientID: o.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		fail(http.StatusUnauthorized, "the ID token is invalid", "error", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(f.Nonce)) != 1 {
		fail(http.StatusBadRequest, "nonce mismatch")
		return
	}
	var claims map[string]any
	if err := idt.Claims(&claims); err != nil {
		fail(http.StatusBadGateway, "the ID token claims cannot be read", "error", err)
		return
	}
	u := userFromClaims(claims, o.cfg.UsernameClaim)
	if !o.allowed(claims) {
		fail(http.StatusForbidden, "you are not a member of a group allowed to use PBOM", "user", u.Name)
		return
	}
	if err := o.cookies.set(w, sessionCookie, "/", session{User: u}, o.cfg.SessionDuration); err != nil {
		http.Error(w, "cannot create session", http.StatusInternalServerError)
		return
	}
	slog.Info("signed in", "user", u.Name)
	http.Redirect(w, r, f.Redirect, http.StatusFound)
}

func (o *oidcAuth) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(r, o.baseURL) {
		http.Error(w, "cross-site sign-out refused", http.StatusForbidden)
		return
	}
	o.cookies.clear(w, sessionCookie, "/")
	target := "/"
	if _, err := o.getProvider(r.Context()); err == nil && o.endSession != "" {
		v := url.Values{"client_id": {o.cfg.ClientID}, "post_logout_redirect_uri": {o.baseURL + "/"}}
		target = o.endSession + "?" + v.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// sameOrigin rejects cross-site requests. Origin is "null" for same-origin
// form posts under Referrer-Policy no-referrer, so Sec-Fetch-Site comes first.
func sameOrigin(r *http.Request, base string) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "null" || origin == base
}

func userFromClaims(c map[string]any, usernameClaim string) User {
	str := func(k string) string {
		s, _ := c[k].(string)
		return s
	}
	name := ""
	if usernameClaim != "" {
		name = str(usernameClaim)
	}
	for _, k := range []string{"name", "preferred_username", "email", "sub"} {
		if name == "" {
			name = str(k)
		}
	}
	return User{Name: name, Email: str("email")}
}

func (o *oidcAuth) allowed(c map[string]any) bool {
	if len(o.cfg.AllowedGroups) == 0 {
		return true
	}
	switch v := c[o.cfg.GroupsClaim].(type) {
	case []any:
		for _, g := range v {
			if s, ok := g.(string); ok && slices.Contains(o.cfg.AllowedGroups, s) {
				return true
			}
		}
	case string:
		return slices.Contains(o.cfg.AllowedGroups, v)
	}
	return false
}
