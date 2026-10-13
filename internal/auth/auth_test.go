package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// fakeIdP is a minimal OpenID provider that checks PKCE and signs ID tokens.
type fakeIdP struct {
	srv       *httptest.Server
	key       *rsa.PrivateKey
	challenge string
	nonce     string
	claims    map[string]any
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIdP{key: key, claims: map[string]any{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		u := idp.srv.URL
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": u, "authorization_endpoint": u + "/authorize", "token_endpoint": u + "/token",
			"jwks_uri": u + "/keys", "end_session_endpoint": u + "/logout",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if r.PostForm.Get("code") != "good-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != idp.challenge {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		claims := map[string]any{
			"iss": idp.srv.URL, "aud": "pbom", "sub": "u-1", "nonce": idp.nonce,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, v := range idp.claims {
			claims[k] = v
		}
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k"}}, (&jose.SignerOptions{}).WithType("JWT"))
		if err != nil {
			t.Error(err)
			return
		}
		payload, _ := json.Marshal(claims)
		jws, _ := signer.Sign(payload)
		raw, _ := jws.CompactSerialize()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": raw})
	})
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func protected(t *testing.T, cfg Config) http.Handler {
	t.Helper()
	mw, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFrom(r.Context())
		_, _ = w.Write([]byte("ok:" + u.Name))
	}))
}

func do(h http.Handler, method, target string, cookies []*http.Cookie, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name && c.MaxAge >= 0 {
			return c
		}
	}
	return nil
}

func oidcConfig(idp *fakeIdP) Config {
	return Config{
		Mode: ModeOIDC, Issuer: idp.srv.URL, ClientID: "pbom",
		RedirectURL: "https://pbom.example.com/auth/callback", SessionSecret: []byte(strings.Repeat("s", 32)),
	}
}

// signIn runs the browser side of the flow and returns the callback response.
func signIn(t *testing.T, h http.Handler, idp *fakeIdP, rd string, tamper func(q url.Values)) *httptest.ResponseRecorder {
	t.Helper()
	login := do(h, http.MethodGet, "/auth/login?rd="+url.QueryEscape(rd), nil, nil)
	if login.Code != http.StatusFound {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	authz, _ := url.Parse(login.Header().Get("Location"))
	q := authz.Query()
	if !strings.HasPrefix(authz.String(), idp.srv.URL+"/authorize") || q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" {
		t.Fatalf("authorize URL %s", authz)
	}
	idp.challenge, idp.nonce = q.Get("code_challenge"), q.Get("nonce")
	cb := url.Values{"code": {"good-code"}, "state": {q.Get("state")}}
	if tamper != nil {
		tamper(cb)
	}
	return do(h, http.MethodGet, "/auth/callback?"+cb.Encode(), []*http.Cookie{cookie(login, flowCookie)}, nil)
}

func TestOIDCSignIn(t *testing.T) {
	idp := newFakeIdP(t)
	idp.claims = map[string]any{"name": "Ada Lovelace", "email": "ada@example.com"}
	h := protected(t, oidcConfig(idp))

	if rec := do(h, http.MethodGet, "/api/matrix", nil, nil); rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"login":"/auth/login"`) {
		t.Errorf("API without session: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/assets/app.js", nil, nil); rec.Code != http.StatusOK || rec.Body.String() != "ok:" {
		t.Errorf("UI shell without session: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/anything", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST without session: %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/healthz", nil, nil); rec.Code != http.StatusOK {
		t.Errorf("healthz: %d", rec.Code)
	}

	cb := signIn(t, h, idp, "/#/releases", nil)
	if cb.Code != http.StatusFound || cb.Header().Get("Location") != "/#/releases" {
		t.Fatalf("callback: %d %s %s", cb.Code, cb.Header().Get("Location"), cb.Body)
	}
	sess := cookie(cb, sessionCookie)
	if sess == nil || !sess.HttpOnly || !sess.Secure || sess.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie %+v", sess)
	}
	if rec := do(h, http.MethodGet, "/api/matrix", []*http.Cookie{sess}, nil); rec.Body.String() != "ok:Ada Lovelace" {
		t.Errorf("API with session: %d %s", rec.Code, rec.Body)
	}
	me := do(h, http.MethodGet, "/api/me", []*http.Cookie{sess}, nil)
	if !strings.Contains(me.Body.String(), `"name":"Ada Lovelace"`) || !strings.Contains(me.Body.String(), `"logout":true`) {
		t.Errorf("me: %s", me.Body)
	}

	out := do(h, http.MethodPost, "/auth/logout", []*http.Cookie{sess}, map[string]string{"Origin": "https://pbom.example.com"})
	if out.Code != http.StatusSeeOther || !strings.HasPrefix(out.Header().Get("Location"), idp.srv.URL+"/logout?") {
		t.Errorf("logout: %d %s", out.Code, out.Header().Get("Location"))
	}
	if rec := do(h, http.MethodPost, "/auth/logout", nil, map[string]string{"Origin": "https://evil.example"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site logout: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/auth/logout", nil, map[string]string{"Origin": "null", "Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site logout with null origin: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/auth/logout", nil, map[string]string{"Origin": "null", "Sec-Fetch-Site": "same-origin"}); rec.Code != http.StatusSeeOther {
		t.Errorf("same-origin logout under no-referrer: %d", rec.Code)
	}
}

func TestOIDCRejects(t *testing.T) {
	idp := newFakeIdP(t)
	h := protected(t, oidcConfig(idp))
	cases := map[string]func(url.Values){
		"state":  func(q url.Values) { q.Set("state", "forged") },
		"code":   func(q url.Values) { q.Set("code", "stolen") },
		"denied": func(q url.Values) { q.Del("code"); q.Set("error", "access_denied") },
	}
	for name, tamper := range cases {
		if rec := signIn(t, h, idp, "/", tamper); rec.Code < 400 || cookie(rec, sessionCookie) != nil {
			t.Errorf("%s: %d, session %v", name, rec.Code, cookie(rec, sessionCookie))
		}
	}
	if rec := do(h, http.MethodGet, "/auth/callback?code=good-code&state=x", nil, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("callback without sign-in cookie: %d", rec.Code)
	}
	forged := &http.Cookie{Name: sessionCookie, Value: "bm90LWEtc2Vzc2lvbg"}
	if rec := do(h, http.MethodGet, "/api/me", []*http.Cookie{forged}, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("forged session: %d", rec.Code)
	}
}

func TestOIDCAllowedGroups(t *testing.T) {
	idp := newFakeIdP(t)
	cfg := oidcConfig(idp)
	cfg.AllowedGroups = []string{"platform-team"}
	h := protected(t, cfg)

	idp.claims = map[string]any{"groups": []string{"other"}}
	if rec := signIn(t, h, idp, "/", nil); rec.Code != http.StatusForbidden {
		t.Errorf("non-member: %d", rec.Code)
	}
	idp.claims = map[string]any{"groups": []string{"other", "platform-team"}}
	if rec := signIn(t, h, idp, "/", nil); rec.Code != http.StatusFound || cookie(rec, sessionCookie) == nil {
		t.Errorf("member: %d", rec.Code)
	}
}

func TestOIDCConfigErrors(t *testing.T) {
	for _, cfg := range []Config{
		{Mode: ModeOIDC, ClientID: "c", RedirectURL: "https://p/auth/callback"},
		{Mode: ModeOIDC, Issuer: "https://i", ClientID: "c", RedirectURL: "https://p/callback"},
		{Mode: ModeOIDC, Issuer: "https://i", ClientID: "c", RedirectURL: "https://p/auth/callback", SessionSecret: []byte("short")},
		{Mode: "ldap"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("New(%+v) succeeded", cfg)
		}
	}
}

func TestHeaderMode(t *testing.T) {
	h := protected(t, Config{Mode: ModeHeader, Header: "X-Forwarded-User", TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
	req := func(remote, user string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		r.RemoteAddr = remote
		if user != "" {
			r.Header.Set("X-Forwarded-User", user)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	if rec := req("10.1.2.3:5555", "ada"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"ada"`) {
		t.Errorf("trusted proxy: %d %s", rec.Code, rec.Body)
	}
	if rec := req("192.168.1.1:5555", "ada"); rec.Code != http.StatusForbidden {
		t.Errorf("untrusted client: %d", rec.Code)
	}
	if rec := req("10.1.2.3:5555", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing header: %d", rec.Code)
	}
}

func TestNoneMode(t *testing.T) {
	h := protected(t, Config{})
	if rec := do(h, http.MethodGet, "/api/me", nil, nil); strings.TrimSpace(rec.Body.String()) != `{"mode":"none"}` {
		t.Errorf("me: %s", rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/matrix", nil, nil); rec.Body.String() != "ok:" {
		t.Errorf("passthrough: %s", rec.Body)
	}
}

func TestSafeRedirect(t *testing.T) {
	for in, want := range map[string]string{
		"/#/releases": "/#/releases", "/a?b=c": "/a?b=c", "": "/", "https://evil.example": "/",
		"//evil.example": "/", "/\\evil.example": "/", "javascript:alert(1)": "/",
	} {
		if got := safeRedirect(in); got != want {
			t.Errorf("safeRedirect(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSealer(t *testing.T) {
	s, _ := newSealer([]byte(strings.Repeat("k", 32)))
	v, _ := seal(s, "a", "value", time.Minute)
	if got, err := unseal[string](s, "a", v); err != nil || got != "value" {
		t.Errorf("round trip: %q %v", got, err)
	}
	if _, err := unseal[string](s, "b", v); err == nil {
		t.Error("cookie accepted under another name")
	}
	old, _ := seal(s, "a", "value", -time.Minute)
	if _, err := unseal[string](s, "a", old); err == nil {
		t.Error("expired cookie accepted")
	}
}
