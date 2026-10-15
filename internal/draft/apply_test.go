package draft

import (
	"strings"
	"testing"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestApply(t *testing.T) {
	p := &api.Platform{Spec: api.PlatformSpec{Environments: []api.Environment{
		{Name: "dev"}, {Name: "prod", TargetRelease: ""}, {Name: "edge", TargetRelease: "0.9.0"},
	}}}
	byEnv := map[string]*api.Inventory{
		"dev":  {Components: []api.DiscoveredComponent{{Name: "argocd", DisplayName: "Argo CD", Category: "delivery", Version: "3.2.0", Known: true}}},
		"prod": {Components: []api.DiscoveredComponent{{Name: "argocd", DisplayName: "Argo CD", Category: "delivery", Version: "3.1.5", Known: true}}},
	}

	out, rels, drafted := Apply(p, nil, byEnv)
	if strings.Join(drafted, ",") != "offerings,description,release,targets" {
		t.Fatalf("drafted = %v", drafted)
	}
	if len(rels) != 1 || rels[0].Metadata.Name != FirstRelease || rels[0].Spec.Components["argocd"] != "3.1.5" || rels[0].Metadata.Labels[Label] != "true" {
		t.Fatalf("release drafted from prod = %+v", rels)
	}
	if out.Spec.Environments[0].TargetRelease != FirstRelease || out.Spec.Environments[2].TargetRelease != "0.9.0" {
		t.Errorf("targets = %+v", out.Spec.Environments)
	}
	if p.Spec.Environments[0].TargetRelease != "" || len(p.Spec.Offerings) != 0 {
		t.Error("Apply modified the configured platform")
	}

	saved := []api.PlatformRelease{{Metadata: api.Metadata{Name: "1.1.0"}}}
	_, rels, drafted = Apply(p, saved, byEnv)
	if len(rels) != 1 || rels[0].Metadata.Name != "1.1.0" || strings.Contains(strings.Join(drafted, ","), "release") {
		t.Errorf("a saved release must not be replaced: %v %v", rels, drafted)
	}

	_, rels, drafted = Apply(p, nil, map[string]*api.Inventory{"dev": {Errors: []string{"unreachable"}}})
	if len(rels) != 0 || len(drafted) != 0 {
		t.Errorf("nothing discovered must draft nothing: %v %v", rels, drafted)
	}
}
