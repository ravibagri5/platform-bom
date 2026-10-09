package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

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
	got := map[string][]string{}
	for _, g := range k.ConfigMaps {
		got[g.Name] = g.Files
	}
	want := map[string][]string{
		"pbom-platform": {"pbom.yaml=platform.yaml", "edge.inventory.yaml"},
		"pbom-releases": {"releases/1.0.0.yaml"},
	}
	if len(got) != len(want) {
		t.Fatalf("generators = %v, want %v", got, want)
	}
	for name, files := range want {
		if strings.Join(got[name], ",") != strings.Join(files, ",") {
			t.Errorf("%s files = %v, want %v", name, got[name], files)
		}
	}
	if k.Namespace != "platform-bom" || !k.GeneratorOptions["disableNameSuffixHash"] {
		t.Errorf("namespace/options = %q/%v", k.Namespace, k.GeneratorOptions)
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
	if err := writePlatform(path, `team "x"`, "prod", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The exact snippet getting-started.md tells users to append.
	_, _ = f.WriteString("    - name: edge\n      inventoryFile: edge.inventory.yaml\n")
	_ = f.Close()
	p, err := service.LoadPlatform(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Metadata.Name != `team "x"` || len(p.Spec.Environments) != 2 || p.Spec.Environments[1].InventoryFile != "edge.inventory.yaml" {
		t.Fatalf("unexpected platform: %+v", p)
	}
	if p.Spec.Environments[0].TargetRelease != "1.0.0" || !p.Spec.Environments[0].InCluster || p.Spec.ComponentsDir != "components" {
		t.Fatalf("unexpected first environment: %+v", p.Spec)
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{"my-platform": "My Platform", "prod": "Prod", "a_b.c": "A B C"} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
