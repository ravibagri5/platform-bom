// Package server exposes the platform-bom HTTP API and web UI.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/draft"
	"github.com/ravibagri5/platform-bom/internal/release"
	"github.com/ravibagri5/platform-bom/internal/service"
)

// New returns the HTTP handler for svc, serving the UI from ui.
func New(svc *service.Service, ui fs.FS) http.Handler {
	h := &handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/platform", h.platform)
	mux.HandleFunc("GET /api/matrix", h.matrix)
	mux.HandleFunc("GET /api/inventory", h.inventory)
	mux.HandleFunc("GET /api/updates", h.updates)
	mux.HandleFunc("GET /api/releases", h.releases)
	mux.HandleFunc("GET /api/releases/{name}", h.release)
	mux.HandleFunc("GET /api/diff", h.diff)
	mux.HandleFunc("GET /api/catalog", h.catalog)
	mux.HandleFunc("GET /api/snapshot", h.snapshot)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	mux.Handle("/", spa(ui))
	return securityHeaders(logRequests(mux))
}

type handler struct{ svc *service.Service }

func refresh(r *http.Request) bool { return r.URL.Query().Get("refresh") == "true" }

type platformResponse struct {
	Platform       *api.Platform        `json:"platform"`
	Readme         string               `json:"readme,omitempty"`
	CurrentRelease *api.PlatformRelease `json:"currentRelease,omitempty"`
	// Drafted lists what was filled in from discovery: description,
	// offerings, release and targets.
	Drafted []string `json:"drafted,omitempty"`
}

// drafted returns the platform and releases with what they leave empty
// drafted from the last discovery, which runs once if none has yet.
func (h *handler) drafted(r *http.Request) (*api.Platform, []api.PlatformRelease, []string, error) {
	rels, err := h.svc.Releases()
	if err != nil {
		return nil, nil, nil, err
	}
	p, rels, drafted := draft.Apply(h.svc.Platform, rels, h.svc.CachedInventories(r.Context()))
	return p, rels, drafted, nil
}

func (h *handler) platform(w http.ResponseWriter, r *http.Request) {
	readme, err := h.svc.Readme()
	if err != nil {
		slog.Warn("cannot read platform readme", "error", err)
	}
	p, rels, drafted, err := h.drafted(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := platformResponse{Platform: p, Readme: readme, Drafted: drafted}
	if len(rels) > 0 {
		resp.CurrentRelease = &rels[0]
	}
	writeJSON(w, resp)
}

// input is the analysis input with drafts applied.
func (h *handler) input(r *http.Request) (analysis.Input, error) {
	in, err := h.svc.Input(r.Context(), refresh(r))
	if err != nil {
		return in, err
	}
	in.Platform, in.Releases, _ = draft.Apply(in.Platform, in.Releases, in.Inventories)
	return in, nil
}

func (h *handler) matrix(w http.ResponseWriter, r *http.Request) {
	in, err := h.input(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, analysis.BuildMatrix(in))
}

func (h *handler) inventory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.svc.Inventories(r.Context(), refresh(r)))
}

func (h *handler) updates(w http.ResponseWriter, r *http.Request) {
	in, err := h.input(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, analysis.BuildUpdates(in))
}

func (h *handler) releases(w http.ResponseWriter, r *http.Request) {
	_, rels, _, err := h.drafted(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rels == nil {
		rels = []api.PlatformRelease{}
	}
	writeJSON(w, rels)
}

func (h *handler) release(w http.ResponseWriter, r *http.Request) {
	_, rels, _, err := h.drafted(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rel := analysis.FindRelease(rels, r.PathValue("name"))
	if rel == nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	writeJSON(w, rel)
}

func (h *handler) diff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d, err := h.svc.Diff(q.Get("from"), q.Get("to"))
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, d)
	}
}

func (h *handler) catalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, h.svc.Catalog.List())
}

// snapshot returns what an environment runs now as a PlatformRelease in YAML,
// for the user to apply; PBOM itself never writes it.
func (h *handler) snapshot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name, env := q.Get("name"), q.Get("env")
	if !release.ValidName(name) || !k8sNameRE.MatchString(name) {
		writeError(w, http.StatusBadRequest, "name must be a release version such as 1.1.0")
		return
	}
	p, _, _, err := h.drafted(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	inv := h.svc.CachedInventories(r.Context())[env]
	if inv == nil {
		writeError(w, http.StatusNotFound, "unknown environment")
		return
	}
	if len(inv.Components) == 0 {
		writeError(w, http.StatusConflict, "nothing was discovered in this environment")
		return
	}
	data, err := yaml.Marshal(draft.Release(name, env, inv, p.Spec.Offerings))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	if q.Get("download") != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.yaml"`)
	}
	_, _ = w.Write(data) //nolint:gosec // YAML of a validated release, served as application/yaml with nosniff
}

// k8sNameRE is a valid Kubernetes object name, which a release is in CRD mode.
var k8sNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$`)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// spa serves static files, falling back to index.html for client routes.
func spa(ui fs.FS) http.Handler {
	files := http.FileServerFS(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(ui, p); err != nil {
			if _, err := fs.Stat(ui, "index.html"); err != nil {
				http.Error(w, "UI not built. Run `make ui` and rebuild, or use the /api endpoints.", http.StatusNotFound)
				return
			}
			r.URL.Path = "/"
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			return
		}
		level := slog.LevelDebug
		if sw.status >= http.StatusInternalServerError {
			level = slog.LevelWarn
		}
		slog.Log(r.Context(), level, "request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
