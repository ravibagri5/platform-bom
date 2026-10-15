package draft

import (
	"strings"
	"testing"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestOfferingsAndDescription(t *testing.T) {
	prod := &api.Inventory{Cluster: api.ClusterInfo{Version: "1.34"}, Components: []api.DiscoveredComponent{
		{Name: "kubernetes", DisplayName: "Kubernetes", Category: "runtime", Known: true},
		{Name: "istio", DisplayName: "Istio", Category: "networking", Known: true},
		{Name: "argocd", DisplayName: "Argo CD", Category: "delivery", Known: true},
		{Name: "provider-aws", Category: "infrastructure", Known: true, PartOf: "crossplane"},
		{Name: "my-app", Category: "other"},
	}}
	dev := &api.Inventory{Components: []api.DiscoveredComponent{
		{Name: "kargo", DisplayName: "Kargo", Category: "delivery", Known: true},
		{Name: "argocd", DisplayName: "Argo CD", Category: "delivery", Known: true},
	}}
	failed := &api.Inventory{Errors: []string{"cannot reach cluster"}}

	offs := Offerings([]*api.Inventory{prod, dev, failed})
	got := make([]string, 0, len(offs))
	for _, o := range offs {
		got = append(got, o.Name+"="+strings.Join(o.Components, ","))
	}
	if want := "runtime=kubernetes delivery=argocd,kargo networking=istio"; strings.Join(got, " ") != want {
		t.Fatalf("offerings = %v, want %s", got, want)
	}
	if !strings.Contains(offs[1].Description, "Provided by Argo CD and Kargo.") {
		t.Errorf("description = %q", offs[1].Description)
	}

	desc := Description([]*api.Inventory{prod, dev, failed}, offs)
	want := "A Kubernetes 1.34 platform running in 2 environments. It gives application teams: Kubernetes workloads; Application delivery; Networking and traffic."
	if desc != want {
		t.Errorf("description = %q", desc)
	}
	if len(Offerings([]*api.Inventory{failed})) != 0 || Description([]*api.Inventory{failed}, nil) != "" {
		t.Error("nothing discovered must draft nothing")
	}
}
