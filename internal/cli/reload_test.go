package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ravibagri5/platform-bom/internal/service"
)

const testPlatform = `apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: %s
spec:
  componentsDir: components
  environments:
    - name: prod
      inventoryFile: prod.yaml
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigFingerprintTracksEveryConfigFile(t *testing.T) {
	dir := t.TempDir()
	platform := filepath.Join(dir, "pbom.yaml")
	writeFile(t, platform, fmt.Sprintf(testPlatform, "one"))
	writeFile(t, filepath.Join(dir, "prod.yaml"), "kind: Inventory\n")
	before := configFingerprint(platform)

	cases := map[string]string{
		"component":   filepath.Join(dir, "components", "x.yaml"),
		"release":     filepath.Join(dir, "releases", "1.0.0.yaml"),
		"inventory":   filepath.Join(dir, "prod.yaml"),
		"ignored-txt": filepath.Join(dir, "components", "notes.txt"),
	}
	for name, path := range cases {
		writeFile(t, path, "changed: "+name+"\n")
		after := configFingerprint(platform)
		if changed := after != before; changed == (name == "ignored-txt") {
			t.Errorf("%s: fingerprint changed=%v", name, changed)
		}
		before = after
	}
}

func TestWatchConfigReloadsValidChangesOnly(t *testing.T) {
	dir := t.TempDir()
	platform := filepath.Join(dir, "pbom.yaml")
	writeFile(t, platform, fmt.Sprintf(testPlatform, "one"))
	writeFile(t, filepath.Join(dir, "prod.yaml"), "kind: Inventory\n")
	writeFile(t, filepath.Join(dir, "components", "README"), "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	swapped := make(chan string, 4)
	load := func() (*service.Service, error) {
		return service.New(platform, service.Options{NoUpstream: true})
	}
	go watchConfig(ctx, platform, 10*time.Millisecond, load, func(s *service.Service) {
		swapped <- s.Platform.Metadata.Name
	})

	writeFile(t, platform, "kind: Platform\nmetadata: {}\n")
	select {
	case name := <-swapped:
		t.Fatalf("invalid configuration was loaded as %q", name)
	case <-time.After(100 * time.Millisecond):
	}

	writeFile(t, platform, fmt.Sprintf(testPlatform, "two"))
	select {
	case name := <-swapped:
		if name != "two" {
			t.Fatalf("reloaded %q, want two", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("valid configuration change was not reloaded")
	}
}
