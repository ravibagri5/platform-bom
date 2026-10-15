package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/service"
)

func TestWriteKustomizationListsPlatformFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "platform.yaml"), "kind: Platform\n")
	writeFile(t, filepath.Join(dir, "edge.inventory.yaml"), "kind: Inventory\n")
	writeFile(t, filepath.Join(dir, "releases", "1.0.0.yaml"), "kind: PlatformRelease\n")
	writeFile(t, filepath.Join(dir, "releases", "notes.txt"), "ignored\n")
	writeFile(t, filepath.Join(dir, "local", "x.yaml"), "ignored: subdirectory\n")

	if _, err := writeKustomization(dir, "platform-bom"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), kustomizeMarker) {
		t.Fatalf("missing marker:\n%s", data)
	}
	var k kustomization
	if err := yaml.Unmarshal(data, &k); err != nil {
		t.Fatal(err)
	}
	if strings.Join(k.Resources, ",") != "platform.yaml,edge.inventory.yaml,releases/1.0.0.yaml" {
		t.Errorf("resources = %v", k.Resources)
	}
	if k.Namespace != "platform-bom" {
		t.Errorf("namespace = %q", k.Namespace)
	}

	writeFile(t, filepath.Join(dir, "components", "gw.yaml"), "kind: Component\n")
	if path, err := syncKustomization(filepath.Join(dir, "platform.yaml")); err != nil || path == "" {
		t.Fatalf("sync = %q, %v", path, err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if !strings.Contains(string(data), "components/gw.yaml") {
		t.Errorf("sync did not add the component:\n%s", data)
	}
}

func TestWriteKustomizationKeepsHandWrittenFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "platform.yaml"), "kind: Platform\n")
	writeFile(t, filepath.Join(dir, "kustomization.yaml"), "resources: []\n")
	if _, err := writeKustomization(dir, "platform-bom"); err == nil {
		t.Fatal("overwrote a kustomization.yaml pbom did not generate")
	}
	if path, err := syncKustomization(filepath.Join(dir, "platform.yaml")); err != nil || path != "" {
		t.Fatalf("sync touched a hand-written kustomization: %q, %v", path, err)
	}
}

func TestWritePlatformAcceptsAppendedEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "platform.yaml")
	offs := []api.Offering{{Name: "runtime", DisplayName: "Kubernetes workloads", Category: "runtime", Status: "ga", Components: []string{"kubernetes"}}}
	if err := writePlatform(path, `team "x"`, "prod", "1.0.0", "A test platform.", offs); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The exact snippet getting-started.md tells users to append.
	_, _ = f.WriteString("    - name: edge\n      inventory: edge\n")
	_ = f.Close()
	p, err := service.LoadPlatform(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Metadata.Name != `team "x"` || len(p.Spec.Environments) != 2 || p.Spec.Environments[1].Inventory != "edge" {
		t.Fatalf("unexpected platform: %+v", p)
	}
	if p.Spec.Environments[0].TargetRelease != "1.0.0" || !p.Spec.Environments[0].InCluster || p.Spec.ComponentsDir != "components" {
		t.Fatalf("unexpected first environment: %+v", p.Spec)
	}
}

func TestInitOfferingsGroupsByCategory(t *testing.T) {
	inv := &api.Inventory{Components: []api.DiscoveredComponent{
		{Name: "kubernetes", DisplayName: "Kubernetes", Category: "runtime", Known: true},
		{Name: "istio", DisplayName: "Istio", Category: "networking", Known: true},
		{Name: "argo-cd", DisplayName: "Argo CD", Category: "delivery", Known: true},
		{Name: "ingress-nginx", DisplayName: "Ingress NGINX", Category: "networking", Known: true},
		{Name: "provider-aws", Category: "infrastructure", Known: true, PartOf: "crossplane"},
		{Name: "my-app", Category: "other", Kind: "helm"},
		{Name: "xdatabases", Category: "platform-api", Known: true},
	}}
	got := initOfferings(inv)
	want := []struct{ name, comps string }{
		{"runtime", "kubernetes"},
		{"delivery", "argo-cd"},
		{"networking", "ingress-nginx,istio"},
	}
	if len(got) != len(want) {
		t.Fatalf("offerings = %+v", got)
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Category != w.name || strings.Join(got[i].Components, ",") != w.comps {
			t.Errorf("offering %d = %+v, want %s %s", i, got[i], w.name, w.comps)
		}
	}
	if got[2].Description != "Expose services, route traffic and secure the connections between them. Provided by Ingress NGINX and Istio." {
		t.Errorf("description = %q", got[2].Description)
	}

	path := filepath.Join(t.TempDir(), "platform.yaml")
	if err := writePlatform(path, "p", "prod", "1.0.0", "", got); err != nil {
		t.Fatal(err)
	}
	p, err := service.LoadPlatform(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Spec.Offerings) != 3 || p.Spec.Offerings[2].Components[1] != "istio" {
		t.Fatalf("round trip = %+v", p.Spec.Offerings)
	}

	if empty := initOfferings(&api.Inventory{}); len(empty) != 1 || empty[0].Name != "runtime" {
		t.Errorf("fallback = %+v", empty)
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{"my-platform": "My Platform", "prod": "Prod", "a_b.c": "A B C"} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
