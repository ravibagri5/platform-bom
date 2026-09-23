package discovery

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestDecodeHelmRelease(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(`{"name":"argocd","chart":{"metadata":{"name":"argo-cd","version":"8.1.0","appVersion":"v3.1.0"}}}`))
	_ = gz.Close()

	rel, err := decodeHelmRelease([]byte(base64.StdEncoding.EncodeToString(buf.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Name != "argocd" || rel.Chart.Metadata.Name != "argo-cd" || rel.Chart.Metadata.AppVersion != "v3.1.0" {
		t.Fatalf("unexpected release: %+v", rel)
	}
}

func TestClassifyPackage(t *testing.T) {
	cases := []struct{ resource, repo, name, display, category, partOf string }{
		{"configurations", "ghcr.io/example/pkgs/postgres", "postgres", "postgres", "platform-api", ""},
		{"functions", "ghcr.io/example/pkgs/postgres_function-render", "postgres_function-render", "function-render", "platform-api", "postgres"},
		{"functions", "ghcr.io/example/pkgs/function-hash", "function-hash", "function-hash", "composition", "crossplane"},
		{"providers", "ghcr.io/example/provider-queue", "provider-queue", "provider-queue", "infrastructure", "crossplane"},
	}
	for _, c := range cases {
		name, display, category, partOf := classifyPackage(c.resource, c.repo)
		if name != c.name || display != c.display || category != c.category || partOf != c.partOf {
			t.Errorf("classifyPackage(%s, %s) = %s %s %s %s", c.resource, c.repo, name, display, category, partOf)
		}
	}
}

func TestKindOf(t *testing.T) {
	cases := map[string][]api.Evidence{
		"function":   {{Source: api.SourceAPIGroup}, {Source: api.SourceCrossplane, Object: "Function/fn"}},
		"controller": {{Source: api.SourceHelm, Object: "HelmRelease/x"}, {Source: api.SourceImage, Object: "Deployment/x"}},
		"helm":       {{Source: api.SourceHelm, Object: "HelmRelease/x"}},
		"api":        {{Source: api.SourceAPIGroup}},
	}
	for want, ev := range cases {
		if got := KindOf(ev); got != want {
			t.Errorf("KindOf = %s, want %s", got, want)
		}
	}
}

func TestResolveVersion(t *testing.T) {
	ev := []api.Evidence{
		{Source: api.SourceHelm, Version: "v3.0.0"},
		{Source: api.SourceImage, Version: "v3.1.0"},
		{Source: api.SourceImage, Version: "v3.1.0"},
		{Source: api.SourceImage, Version: "v3.1.1"},
		{Source: api.SourceAPIGroup},
	}
	if got := ResolveVersion(ev); got != "3.1.0" {
		t.Fatalf("got %q, want 3.1.0", got)
	}
	if got := ResolveVersion([]api.Evidence{{Source: api.SourceImage, Version: "latest"}, {Source: api.SourceHelm, Version: "1.2.3"}}); got != "1.2.3" {
		t.Fatalf("got %q, want fallback to helm 1.2.3", got)
	}
}
