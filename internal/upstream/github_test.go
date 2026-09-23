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

func TestReleasesRejectsBadSlug(t *testing.T) {
	if _, err := NewClient().Releases(context.Background(), &api.Upstream{GitHub: "../../etc"}); err == nil {
		t.Fatal("expected error for invalid slug")
	}
}
