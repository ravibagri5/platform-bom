package analysis

import (
	"testing"
	"time"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/upstream"
)

func rel(name string, comps map[string]string) api.PlatformRelease {
	return api.PlatformRelease{Metadata: api.Metadata{Name: name}, Spec: api.ReleaseSpec{Version: name, Components: comps}}
}

func inv(env string, comps map[string]string) *api.Inventory {
	i := &api.Inventory{Environment: env}
	for n, v := range comps {
		i.Components = append(i.Components, api.DiscoveredComponent{Name: n, Version: v, Known: true})
	}
	return i
}

func testInput(t *testing.T) Input {
	t.Helper()
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return Input{
		Platform: &api.Platform{Spec: api.PlatformSpec{Environments: []api.Environment{
			{Name: "dev", TargetRelease: "1.1.0"},
			{Name: "prod", TargetRelease: "1.1.0"},
		}}},
		Catalog: cat,
		Releases: []api.PlatformRelease{
			rel("1.1.0", map[string]string{"kubernetes": "1.34", "argocd": "3.1.0", "crossplane": "2.0.2"}),
			rel("1.0.0", map[string]string{"kubernetes": "1.33", "argocd": "3.0.0"}),
		},
		Inventories: map[string]*api.Inventory{
			"dev":  inv("dev", map[string]string{"kubernetes": "1.34.1", "argocd": "3.1.0", "crossplane": "2.0.2", "kyverno": "1.15.0"}),
			"prod": inv("prod", map[string]string{"kubernetes": "1.33.4", "argocd": "3.0.0"}),
		},
		Upstream: map[string][]upstream.Release{
			"kubernetes": {{Version: "1.36.0", PublishedAt: time.Now()}, {Version: "1.35.2"}, {Version: "1.34.3"}, {Version: "1.33.6"}, {Version: "1.33.4"}},
			"argocd":     {{Version: "3.1.0"}, {Version: "3.0.0"}},
		},
	}
}

func TestBuildMatrix(t *testing.T) {
	m := BuildMatrix(testInput(t))
	envs := map[string]EnvStatus{}
	for _, e := range m.Environments {
		envs[e.Name] = e
	}
	if !envs["dev"].Compliant || envs["dev"].Untracked != 1 || envs["dev"].MatchedRelease != "1.1.0" {
		t.Errorf("dev: %+v", envs["dev"])
	}
	if p := envs["prod"]; p.Compliant || p.Drift != 2 || p.Missing != 1 || p.MatchedRelease != "1.0.0" {
		t.Errorf("prod: %+v", p)
	}
	for _, r := range m.Rows {
		if r.Component == "kubernetes" && r.Freshness != FreshUnsupported {
			t.Errorf("kubernetes freshness = %s, want unsupported (prod 3 minors behind)", r.Freshness)
		}
	}
	if m.Summary.Alignment != 50 {
		t.Errorf("alignment = %d, want 50", m.Summary.Alignment)
	}
}

func TestBuildUpdates(t *testing.T) {
	ups := BuildUpdates(testInput(t))
	if len(ups) == 0 || ups[0].Component != "kubernetes" {
		t.Fatalf("expected kubernetes first, got %+v", ups)
	}
	k := ups[0]
	if k.Current != "1.33.4" || k.Freshness != FreshUnsupported || k.MinorsBehind != 3 || len(k.Newer) != 4 {
		t.Errorf("unexpected kubernetes update: %+v", k)
	}
}
