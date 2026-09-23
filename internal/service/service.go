// Package service wires configuration, discovery, upstream data and analysis
// together. It is shared by the CLI and the HTTP server.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/discovery"
	"github.com/ravibagri5/platform-bom/internal/release"
	"github.com/ravibagri5/platform-bom/internal/upstream"
)

var envNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

const upstreamConcurrency = 4

// Options configure a Service.
type Options struct {
	// NoUpstream disables fetching upstream releases.
	NoUpstream bool
	// CacheTTL is how long discovered inventories are reused.
	CacheTTL time.Duration
}

// Service is the platform-bom application core.
type Service struct {
	Platform *api.Platform
	Catalog  *catalog.Catalog
	baseDir  string
	upstream *upstream.Client
	ttl      time.Duration

	refreshMu sync.Mutex
	mu        sync.RWMutex
	invs      map[string]*api.Inventory
	invsAt    time.Time
	ups       map[string][]upstream.Release
	upsErrs   map[string]string
	upsAt     time.Time
}

// LoadPlatform reads and validates a Platform document.
func LoadPlatform(path string) (*api.Platform, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := &api.Platform{}
	if err := yaml.UnmarshalStrict(data, p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.Kind != api.KindPlatform {
		return nil, fmt.Errorf("%s: expected kind %s, got %q", path, api.KindPlatform, p.Kind)
	}
	if p.Metadata.Name == "" {
		return nil, fmt.Errorf("%s: metadata.name is required", path)
	}
	seen := map[string]bool{}
	for _, e := range p.Spec.Environments {
		if !envNameRE.MatchString(e.Name) {
			return nil, fmt.Errorf("%s: invalid environment name %q", path, e.Name)
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("%s: duplicate environment %q", path, e.Name)
		}
		seen[e.Name] = true
	}
	if p.Spec.ReleasesDir == "" {
		p.Spec.ReleasesDir = "releases"
	}
	return p, nil
}

// New loads the platform at path.
func New(path string, opts Options) (*Service, error) {
	p, err := LoadPlatform(path)
	if err != nil {
		return nil, err
	}
	s := &Service{Platform: p, baseDir: filepath.Dir(path), ttl: opts.CacheTTL}
	if s.ttl == 0 {
		s.ttl = 5 * time.Minute
	}
	if s.Catalog, err = catalog.Load(s.resolve(p.Spec.ComponentsDir)); err != nil {
		return nil, err
	}
	if !opts.NoUpstream {
		s.upstream = upstream.NewClient()
	}
	return s, nil
}

func (s *Service) resolve(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.baseDir, p)
}

// ReleasesDir is the absolute directory of release documents.
func (s *Service) ReleasesDir() string { return s.resolve(s.Platform.Spec.ReleasesDir) }

// Releases loads releases from disk, newest first.
func (s *Service) Releases() ([]api.PlatformRelease, error) { return release.Load(s.ReleasesDir()) }

// Readme returns the platform README markdown.
func (s *Service) Readme() (string, error) {
	if s.Platform.Spec.ReadmeFile == "" {
		return s.Platform.Spec.Readme, nil
	}
	data, err := os.ReadFile(s.resolve(s.Platform.Spec.ReadmeFile))
	return string(data), err
}

// Environment returns the named environment.
func (s *Service) Environment(name string) (*api.Environment, error) {
	for i := range s.Platform.Spec.Environments {
		if s.Platform.Spec.Environments[i].Name == name {
			return &s.Platform.Spec.Environments[i], nil
		}
	}
	return nil, fmt.Errorf("unknown environment %q", name)
}

// DiscoverEnvironment discovers a single environment, bypassing the cache.
func (s *Service) DiscoverEnvironment(ctx context.Context, env *api.Environment) (*api.Inventory, error) {
	if env.InventoryFile != "" {
		return LoadInventory(s.resolve(env.InventoryFile), env.Name)
	}
	var cfg *rest.Config
	var err error
	contextName := env.KubeContext
	if env.InCluster {
		cfg, err = discovery.InClusterConfig()
		contextName = "in-cluster"
	} else {
		cfg, err = discovery.RESTConfig(s.resolve(env.Kubeconfig), env.KubeContext)
	}
	if err != nil {
		return nil, err
	}
	return discovery.Discover(ctx, cfg, s.Catalog, discovery.Options{
		Environment: env.Name,
		Context:     contextName,
		Policy:      s.Platform.Spec.Discovery,
	})
}

// LoadInventory reads an exported Inventory document.
func LoadInventory(path, env string) (*api.Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	inv := &api.Inventory{}
	if err := yaml.Unmarshal(data, inv); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if inv.Kind != api.KindInventory {
		return nil, fmt.Errorf("%s: expected kind %s", path, api.KindInventory)
	}
	inv.Environment = env
	return inv, nil
}

// Inventories returns inventories for all environments, rediscovering when
// the cache is stale or refresh is set. Unreachable environments are
// returned with errors rather than failing the whole call.
func (s *Service) Inventories(ctx context.Context, refresh bool) map[string]*api.Inventory {
	s.mu.RLock()
	if !refresh && s.invs != nil && time.Since(s.invsAt) < s.ttl {
		defer s.mu.RUnlock()
		return s.invs
	}
	s.mu.RUnlock()

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.mu.RLock()
	fresh := s.invs != nil && time.Since(s.invsAt) < time.Second
	s.mu.RUnlock()
	if fresh {
		return s.invs
	}

	out := make(map[string]*api.Inventory, len(s.Platform.Spec.Environments))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range s.Platform.Spec.Environments {
		env := &s.Platform.Spec.Environments[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			inv, err := s.DiscoverEnvironment(ctx, env)
			if err != nil {
				inv = &api.Inventory{
					TypeMeta:    api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindInventory},
					Metadata:    api.Metadata{Name: env.Name},
					Environment: env.Name,
					CollectedAt: time.Now().UTC(),
					Errors:      []string{err.Error()},
				}
			}
			mu.Lock()
			out[env.Name] = inv
			mu.Unlock()
		}()
	}
	wg.Wait()

	s.mu.Lock()
	s.invs, s.invsAt = out, time.Now()
	s.mu.Unlock()
	return out
}

// upstreamFor fetches upstream releases for every component that appears in
// an inventory or release.
func (s *Service) upstreamFor(ctx context.Context, names map[string]bool, refresh bool) (map[string][]upstream.Release, map[string]string) {
	if s.upstream == nil {
		return nil, nil
	}
	s.mu.RLock()
	if !refresh && s.ups != nil && time.Since(s.upsAt) < s.ttl && covers(s.ups, s.upsErrs, names, s.Catalog) {
		defer s.mu.RUnlock()
		return s.ups, s.upsErrs
	}
	s.mu.RUnlock()

	ups := map[string][]upstream.Release{}
	errs := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, upstreamConcurrency)
	for name := range names {
		comp, ok := s.Catalog.Get(name)
		if !ok || comp.Spec.Upstream == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rels, err := s.upstream.Releases(ctx, comp.Spec.Upstream)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[name] = err.Error()
				return
			}
			ups[name] = rels
		}()
	}
	wg.Wait()

	s.mu.Lock()
	s.ups, s.upsErrs, s.upsAt = ups, errs, time.Now()
	s.mu.Unlock()
	return ups, errs
}

func covers(ups map[string][]upstream.Release, errs map[string]string, names map[string]bool, cat *catalog.Catalog) bool {
	for n := range names {
		c, ok := cat.Get(n)
		if !ok || c.Spec.Upstream == nil {
			continue
		}
		if _, ok := ups[n]; ok {
			continue
		}
		if _, ok := errs[n]; ok {
			continue
		}
		return false
	}
	return true
}

// Input gathers everything needed for analysis.
func (s *Service) Input(ctx context.Context, refresh bool) (analysis.Input, error) {
	rels, err := s.Releases()
	if err != nil {
		return analysis.Input{}, err
	}
	invs := s.Inventories(ctx, refresh)
	names := map[string]bool{}
	for _, inv := range invs {
		for _, c := range inv.Components {
			names[c.Name] = true
		}
	}
	for _, r := range rels {
		for n := range r.Spec.Components {
			names[n] = true
		}
	}
	ups, errs := s.upstreamFor(ctx, names, refresh)
	return analysis.Input{
		Platform: s.Platform, Catalog: s.Catalog, Releases: rels,
		Inventories: invs, Upstream: ups, UpstreamErrors: errs,
	}, nil
}

// ErrNotFound is returned for unknown releases.
var ErrNotFound = errors.New("not found")

// Diff compares two named releases.
func (s *Service) Diff(from, to string) (*release.Diff, error) {
	rels, err := s.Releases()
	if err != nil {
		return nil, err
	}
	a, b := analysis.FindRelease(rels, from), analysis.FindRelease(rels, to)
	if a == nil || b == nil {
		return nil, fmt.Errorf("release %w: %s or %s", ErrNotFound, from, to)
	}
	d := release.Compare(a, b, s.Catalog)
	return &d, nil
}
