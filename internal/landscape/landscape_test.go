package landscape

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSnapshotWorksOffline(t *testing.T) {
	if Source() != "snapshot" || Count() < minProjects {
		t.Fatalf("source %q with %d projects", Source(), Count())
	}
	cases := map[string]string{
		"spiderpool":             "spiderpool",
		"volcano":                "volcano",
		"chaos-mesh":             "chaos-mesh",
		"strimzi-kafka-operator": "strimzi",
		"kubearmor-operator":     "kubearmor",
	}
	for chart, want := range cases {
		if p := Match(chart); p == nil || p.Key != want {
			t.Errorf("Match(%q) = %v, want %s", chart, p, want)
		}
	}
	if p := Match("my-internal-app"); p != nil {
		t.Errorf("unexpected match %s", p.Key)
	}
	c := Component(Match("volcano"))
	if c.Spec.Category != "runtime" || c.Metadata.Labels[SourceLabel] != "cncf-landscape" || c.Spec.Upstream == nil {
		t.Errorf("component = %+v", c)
	}
}

func fixture(n int) string {
	var b strings.Builder
	b.WriteString("landscape:\n  - category:\n    name: Observability and Analysis\n    subcategories:\n      - subcategory:\n        name: Observability\n        items:\n")
	for i := range n {
		fmt.Fprintf(&b, "          - item:\n            name: Tool %d\n            project: sandbox\n            repo_url: https://github.com/example/tool-%d\n", i, i)
	}
	b.WriteString("          - item:\n            name: Vendor product\n            homepage_url: https://example.com\n")
	return b.String()
}

func TestRefreshKeepsSnapshotOnFailure(t *testing.T) {
	saved := current.Load()
	t.Cleanup(func() { current.Store(saved) })

	bodies := map[string]string{
		"/broken":   "{not yaml",
		"/changed":  "projects: []\n",
		"/too-few":  fixture(10),
		"/complete": fixture(150),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	for _, path := range []string{"/missing", "/broken", "/changed", "/too-few"} {
		if _, err := Refresh(context.Background(), srv.URL+path); err == nil {
			t.Errorf("%s: expected an error", path)
		}
		if Source() != "snapshot" || Match("volcano") == nil {
			t.Fatalf("%s: snapshot was replaced", path)
		}
	}
	if _, err := Refresh(context.Background(), "http://127.0.0.1:1/unreachable"); err == nil {
		t.Error("unreachable: expected an error")
	}

	n, err := Refresh(context.Background(), srv.URL+"/complete")
	if err != nil || n != 150 || Source() != srv.URL+"/complete" {
		t.Fatalf("refresh = %d, %v, source %q", n, err, Source())
	}
	p := Match("tool-7")
	if p == nil || p.Category != "observability" || p.GitHub != "example/tool-7" || p.Maturity != "sandbox" {
		t.Errorf("tool-7 = %+v", p)
	}
	if Match("vendor-product") != nil {
		t.Error("non-CNCF landscape items must be ignored")
	}
}
