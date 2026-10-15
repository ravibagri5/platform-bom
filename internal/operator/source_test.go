package operator

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ravibagri5/platform-bom/internal/api"
)

func TestResourcesFromObjects(t *testing.T) {
	platform := resource(api.KindPlatform, "acme", map[string]any{
		"environments": []any{map[string]any{"name": "prod", "inCluster": true}},
	}, map[string]string{DisplayNameAnnotation: "Acme platform"})
	release := resource(api.KindRelease, "1.0.0", map[string]any{
		"version": "1.0.0", "components": map[string]any{"custom": "2.3.4"},
	}, nil)
	component := resource(api.KindComponent, "custom", map[string]any{
		"displayName": "Custom", "category": "compute",
		"discovery": map[string]any{"helmCharts": []any{"custom-*"}},
	}, nil)

	inventory := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": api.APIVersion, "kind": api.KindInventory,
		"metadata":    map[string]any{"name": "edge"},
		"environment": "edge", "collectedAt": "2026-10-15T10:00:00Z",
		"components": []any{map[string]any{"name": "argocd", "version": "3.1.0", "known": true}},
	}}

	got, err := resourcesFromObjects([]*unstructured.Unstructured{platform}, []*unstructured.Unstructured{release}, []*unstructured.Unstructured{component}, []*unstructured.Unstructured{inventory}, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform.Metadata.DisplayName != "Acme platform" || got.Platform.Spec.Environments[0].Name != "prod" {
		t.Fatalf("platform = %#v", got.Platform)
	}
	if len(got.Releases) != 1 || got.Releases[0].Spec.Components["custom"] != "2.3.4" {
		t.Fatalf("releases = %#v", got.Releases)
	}
	if len(got.Components) != 1 || got.Components[0].Metadata.Name != "custom" {
		t.Fatalf("components = %#v", got.Components)
	}
	if len(got.Inventories) != 1 || got.Inventories[0].Metadata.Name != "edge" || got.Inventories[0].Components[0].Version != "3.1.0" || got.Inventories[0].CollectedAt.IsZero() {
		t.Fatalf("inventories = %#v", got.Inventories)
	}
}

func TestResourcesFromObjectsRequiresPlatformSelection(t *testing.T) {
	platforms := []*unstructured.Unstructured{
		resource(api.KindPlatform, "one", map[string]any{}, nil),
		resource(api.KindPlatform, "two", map[string]any{}, nil),
	}
	if _, err := resourcesFromObjects(platforms, nil, nil, nil, ""); err == nil {
		t.Fatal("expected multiple-platform error")
	}
	if _, err := resourcesFromObjects(platforms, nil, nil, nil, "missing"); err == nil {
		t.Fatal("expected missing-platform error")
	}
}

func resource(kind, name string, spec map[string]any, annotations map[string]string) *unstructured.Unstructured {
	objectMetadata := map[string]any{"name": name}
	if annotations != nil {
		objectMetadata["annotations"] = map[string]any{DisplayNameAnnotation: annotations[DisplayNameAnnotation]}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": api.APIVersion,
		"kind":       kind,
		"metadata":   objectMetadata,
		"spec":       spec,
	}}
}
