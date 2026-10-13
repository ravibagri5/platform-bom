package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ravibagri5/platform-bom/internal/service"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	svc, err := service.New("../../examples/acme/pbom.yaml", service.Options{NoUpstream: true})
	if err != nil {
		t.Fatal(err)
	}
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><div id=root></div>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	return New(svc, ui)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSecurityHeaders(t *testing.T) {
	h := newTestServer(t)
	for _, path := range []string{"/", "/api/platform", "/api/unknown", "/assets/app.js"} {
		hdr := get(h, path).Header()
		for name, want := range map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"Referrer-Policy":        "no-referrer",
		} {
			if got := hdr.Get(name); got != want {
				t.Errorf("%s: %s = %q, want %q", path, name, got, want)
			}
		}
		csp := hdr.Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") || strings.Contains(csp, "script-src") {
			t.Errorf("%s: Content-Security-Policy %q", path, csp)
		}
	}
}

func TestAPI(t *testing.T) {
	h := newTestServer(t)
	cases := []struct {
		path string
		code int
		want string
	}{
		{"/healthz", http.StatusOK, "ok"},
		{"/api/platform", http.StatusOK, `"name":"acme-platform"`},
		{"/api/releases", http.StatusOK, `"version":"1.2.0"`},
		{"/api/releases/1.0.0", http.StatusOK, `"version":"1.0.0"`},
		{"/api/releases/9.9.9", http.StatusNotFound, `"error":"release not found"`},
		{"/api/diff?from=1.0.0&to=1.2.0", http.StatusOK, `"from":"1.0.0"`},
		{"/api/diff?from=1.0.0&to=9.9.9", http.StatusNotFound, `"error"`},
		{"/api/matrix", http.StatusOK, `"environments"`},
		{"/api/inventory", http.StatusOK, `"dev"`},
		{"/api/catalog", http.StatusOK, `"kubernetes"`},
		{"/api/unknown", http.StatusNotFound, `"error":"not found"`},
	}
	for _, c := range cases {
		rec := get(h, c.path)
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("GET %s = %d %.200s, want %d containing %s", c.path, rec.Code, rec.Body, c.code, c.want)
		}
		if strings.HasPrefix(c.path, "/api/") && rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("GET %s: Content-Type %q", c.path, rec.Header().Get("Content-Type"))
		}
	}
}

func TestAPIRejectsWrites(t *testing.T) {
	h := newTestServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/releases", strings.NewReader("{}")))
		if rec.Code < 400 {
			t.Errorf("%s /api/releases = %d, want an error", method, rec.Code)
		}
	}
}

func TestMatrixJSON(t *testing.T) {
	var m struct {
		Environments []struct {
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := json.Unmarshal(get(newTestServer(t), "/api/matrix").Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Environments) == 0 {
		t.Error("matrix has no environments")
	}
}

func TestSPA(t *testing.T) {
	h := newTestServer(t)
	if rec := get(h, "/releases/1.0.0"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "id=root") {
		t.Errorf("client route = %d %s, want index.html", rec.Code, rec.Body)
	}
	asset := get(h, "/assets/app.js")
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset = %d, Cache-Control %q", asset.Code, asset.Header().Get("Cache-Control"))
	}
	if rec := get(h, "/../../../etc/passwd"); strings.Contains(rec.Body.String(), "root:") {
		t.Error("path traversal served a file outside the UI")
	}
	if rec := get(h, "/index.html"); rec.Header().Get("Cache-Control") != "" {
		t.Errorf("index.html must not be cached immutably: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestSPAWithoutBuiltUI(t *testing.T) {
	svc, err := service.New("../../examples/acme/pbom.yaml", service.Options{NoUpstream: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(New(svc, fstest.MapFS{}), "/")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "UI not built") {
		t.Errorf("empty UI = %d %s", rec.Code, rec.Body)
	}
}
