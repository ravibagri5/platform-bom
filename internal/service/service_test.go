package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestLoadPlatformTokenSource(t *testing.T) {
	cases := map[string]string{
		"server: https://qa.example.com:6443\n      tokenFile: /tokens/qa/token\n      caFile: /tokens/qa/ca.crt": "",
		"tokenFile: /tokens/qa/token":                                               "need server",
		"server: http://qa.example.com":                                             "https://",
		"server: https://qa.example.com":                                            "needs tokenFile",
		"server: https://qa.example.com\n      tokenFile: t\n      inCluster: true": "cannot be combined",
	}
	for env, want := range cases {
		path := filepath.Join(t.TempDir(), "pbom.yaml")
		doc := "apiVersion: pbom.dev/v1alpha1\nkind: Platform\nmetadata:\n  name: p\nspec:\n  environments:\n    - name: qa\n      " + env + "\n"
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadPlatform(path)
		switch {
		case want == "" && err != nil:
			t.Errorf("%q: unexpected error %v", env, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%q: error %v, want %q", env, err, want)
		}
	}
}

func TestNewFromResources(t *testing.T) {
	component := &api.Component{
		TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindComponent},
		Metadata: api.Metadata{Name: "custom"},
		Spec:     api.ComponentSpec{DisplayName: "Custom component", Category: "compute"},
	}
	resources := Resources{
		Platform: &api.Platform{
			TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindPlatform},
			Metadata: api.Metadata{Name: "test-platform"},
		},
		Releases: []api.PlatformRelease{{
			TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindRelease},
			Metadata: api.Metadata{Name: "1.0.0"},
			Spec:     api.ReleaseSpec{Components: map[string]string{"custom": "1.2.3"}},
		}},
		Components: []*api.Component{component},
	}

	svc, err := NewFromResources(resources, Options{NoUpstream: true})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Platform.Metadata.Name != "test-platform" {
		t.Fatalf("platform name = %q", svc.Platform.Metadata.Name)
	}
	releases, err := svc.Releases()
	if err != nil || len(releases) != 1 || releases[0].Spec.Version != "1.0.0" {
		t.Fatalf("releases = %#v, err = %v", releases, err)
	}
	got, ok := svc.Catalog.Get("custom")
	if !ok || got.Spec.DisplayName != "Custom component" || got.Spec.Category != "infrastructure" {
		t.Fatalf("custom component = %#v, found = %t", got, ok)
	}
}

func TestNewFromResourcesInventories(t *testing.T) {
	platform := func(env api.Environment) *api.Platform {
		return &api.Platform{
			TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindPlatform},
			Metadata: api.Metadata{Name: "p"},
			Spec:     api.PlatformSpec{Environments: []api.Environment{env}},
		}
	}
	inv := &api.Inventory{
		TypeMeta:   api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindInventory},
		Metadata:   api.Metadata{Name: "edge-export"},
		Components: []api.DiscoveredComponent{{Name: "argocd", Category: "compute"}},
	}
	svc, err := NewFromResources(Resources{
		Platform:    platform(api.Environment{Name: "edge", Inventory: "edge-export"}),
		Inventories: []*api.Inventory{inv},
	}, Options{NoUpstream: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.DiscoverEnvironment(context.Background(), &svc.Platform.Spec.Environments[0])
	if err != nil || got.Environment != "edge" || got.Components[0].Category != "infrastructure" {
		t.Fatalf("inventory = %#v, err = %v", got, err)
	}
	if inv.Components[0].Category != "compute" {
		t.Fatal("the watched Inventory was modified")
	}
	missing := api.Environment{Name: "x", Inventory: "missing"}
	if _, err := svc.DiscoverEnvironment(context.Background(), &missing); err == nil {
		t.Fatal("expected missing-inventory error")
	}

	if _, err := NewFromResources(Resources{Platform: platform(api.Environment{Name: "edge", InventoryFile: "edge.yaml"})}, Options{NoUpstream: true}); err == nil {
		t.Fatal("inventoryFile must be refused in a Platform resource")
	}
}
