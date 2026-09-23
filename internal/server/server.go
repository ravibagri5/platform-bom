// Package server exposes the platform-bom HTTP API and web UI.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/service"
)

// New returns the HTTP handler for svc, serving the UI from ui.
func New(svc *service.Service, ui fs.FS) http.Handler {
	h := &handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/platform", h.platform)
	mux.HandleFunc("GET /api/matrix", h.matrix)
	mux.HandleFunc("GET /api/inventory", h.inventory)
	mux.HandleFunc("GET /api/updates", h.updates)
	mux.HandleFunc("GET /api/releases", h.releases)
	mux.HandleFunc("GET /api/releases/{name}", h.release)
	mux.HandleFunc("GET /api/diff", h.diff)
	mux.HandleFunc("GET /api/catalog", h.catalog)
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
}

func (h *handler) platform(w http.ResponseWriter, _ *http.Request) {
	readme, err := h.svc.Readme()
	if err != nil {
		slog.Warn("cannot read platform readme", "error", err)
	}
	rels, err := h.svc.Releases()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := platformResponse{Platform: h.svc.Platform, Readme: readme}
	if len(rels) > 0 {
		resp.CurrentRelease = &rels[0]
	}
	writeJSON(w, resp)
}

func (h *handler) matrix(w http.ResponseWriter, r *http.Request) {
	in, err := h.svc.Input(r.Context(), refresh(r))
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
	in, err := h.svc.Input(r.Context(), refresh(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, analysis.BuildUpdates(in))
}

func (h *handler) releases(w http.ResponseWriter, _ *http.Request) {
	rels, err := h.svc.Releases()
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
	rels, err := h.svc.Releases()
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
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			slog.Debug("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
		}
	})
}
