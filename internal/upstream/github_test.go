package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestReleasesFiltersAndSorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/kubernetes/ingress-nginx/releases" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"tag_name":"helm-chart-4.13.0","published_at":"2026-09-01T00:00:00Z"},
			{"tag_name":"controller-v1.13.0","published_at":"2026-09-01T00:00:00Z"},
			{"tag_name":"controller-v1.14.0-beta.0","prerelease":true,"published_at":"2026-09-10T00:00:00Z"},
			{"tag_name":"controller-v1.12.5","published_at":"2026-08-01T00:00:00Z"},
			{"tag_name":"controller-v1.13.1","draft":true}
		]`))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	c.CacheDir = ""
	c.TTL = time.Minute

	got, err := c.Releases(context.Background(), &api.Upstream{GitHub: "kubernetes/ingress-nginx", TagPrefix: "controller-v"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Version != "1.13.0" || got[1].Version != "1.12.5" {
		t.Fatalf("unexpected releases: %+v", got)
	}
}

func TestReleasesFallBackToFeedWhenRateLimited(t *testing.T) {
	apiCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/argoproj/argo-cd/releases", "/repos/kyverno/kyverno/releases":
			apiCalls++
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		case "/argoproj/argo-cd/releases.atom", "/kyverno/kyverno/releases.atom":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry><updated>2026-10-15T14:58:49Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/argoproj/argo-cd/releases/tag/v3.6.0-rc3"/><title>v3.6.0-rc3</title></entry>
  <entry><updated>2026-10-01T10:00:00Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/argoproj/argo-cd/releases/tag/v3.5.2"/><title>v3.5.2</title></entry>
</feed>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL, c.WebURL, c.CacheDir = srv.URL, srv.URL, ""
	for _, slug := range []string{"argoproj/argo-cd", "kyverno/kyverno"} {
		got, err := c.Releases(context.Background(), &api.Upstream{GitHub: slug})
		if err != nil || len(got) != 1 || got[0].Version != "3.5.2" {
			t.Fatalf("%s: releases = %+v, %v", slug, got, err)
		}
	}
	if apiCalls != 1 {
		t.Errorf("API called %d times; once rate limited it must not be retried", apiCalls)
	}
}

func TestReleasesRejectsBadSlug(t *testing.T) {
	if _, err := NewClient().Releases(context.Background(), &api.Upstream{GitHub: "../../etc"}); err == nil {
		t.Fatal("expected error for invalid slug")
	}
}
