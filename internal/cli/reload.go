package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ravibagri5/platform-bom/internal/service"
)

// configFingerprint hashes every file serve reads its configuration from:
// the platform file, release and component definitions, the readme and
// exported inventories. Mounted ConfigMaps update these files in place.
func configFingerprint(platformPath, releasesDir, componentsDir string) string {
	h := sha256.New()
	hashFile(h, platformPath)
	p, err := service.LoadPlatform(platformPath)
	if err == nil {
		if releasesDir != "" {
			p.Spec.ReleasesDir = releasesDir
		}
		if componentsDir != "" {
			p.Spec.ComponentsDir = componentsDir
		}
		base := filepath.Dir(platformPath)
		resolve := func(path string) string {
			if path == "" || filepath.IsAbs(path) {
				return path
			}
			return filepath.Join(base, path)
		}
		hashYAMLDir(h, resolve(p.Spec.ReleasesDir))
		hashYAMLDir(h, resolve(p.Spec.ComponentsDir))
		if p.Spec.ReadmeFile != "" {
			hashFile(h, resolve(p.Spec.ReadmeFile))
		}
		for _, env := range p.Spec.Environments {
			if env.InventoryFile != "" {
				hashFile(h, resolve(env.InventoryFile))
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashFile(h hash.Hash, path string) {
	h.Write([]byte(path + "\x00"))
	if data, err := os.ReadFile(path); err == nil {
		h.Write(data)
	}
	h.Write([]byte{0})
}

func hashYAMLDir(h hash.Hash, dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		hashFile(h, filepath.Join(dir, n))
	}
}

// watchConfig reloads the service when its configuration files change. An
// invalid change is logged and the previous configuration keeps serving.
func watchConfig(ctx context.Context, g *globals, every time.Duration, swap func(*service.Service)) {
	last := configFingerprint(g.config, g.releasesDir, g.componentsDir)
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		fp := configFingerprint(g.config, g.releasesDir, g.componentsDir)
		if fp == last {
			continue
		}
		last = fp
		svc, err := g.service()
		if err != nil {
			slog.Warn("configuration changed but is invalid; still serving the previous configuration", "error", err)
			continue
		}
		swap(svc)
		slog.Info("configuration reloaded", "platform", svc.Platform.Metadata.Name)
	}
}
