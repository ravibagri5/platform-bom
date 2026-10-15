// Package service wires configuration, discovery, upstream data and analysis
// together. It is shared by the CLI and the HTTP server.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/discovery"
	"github.com/ravibagri5/platform-bom/internal/metrics"
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
	// ReleasesDir and ComponentsDir override the platform file's settings,
	// for example where the Helm chart mounts them.
	ReleasesDir   string
	ComponentsDir string
}

// Resources are the desired-state objects read from the Kubernetes API.
type Resources struct {
	Platform    *api.Platform
	Releases    []api.PlatformRelease
	Components  []*api.Component
	Inventories []*api.Inventory
}

// Service is the platform-bom application core.
type Service struct {
	Platform            *api.Platform
	Catalog             *catalog.Catalog
	baseDir             string
	upstream            *upstream.Client
	ttl                 time.Duration
	resourcesReleases   []api.PlatformRelease
	useResourceReleases bool
	inventories         map[string]*api.Inventory

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
	if err := validatePlatform(p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

func validatePlatform(p *api.Platform) error {
	if p == nil {
		return errors.New("platform is required")
	}
	if p.Kind != api.KindPlatform {
		return fmt.Errorf("expected kind %s, got %q", api.KindPlatform, p.Kind)
	}
	if p.Metadata.Name == "" {
		return errors.New("metadata.name is required")
	}
	if p.Metadata.DisplayName == "" {
		p.Metadata.DisplayName = p.Metadata.Annotations[api.DisplayNameAnnotation]
	}
	seen := map[string]bool{}
	for _, e := range p.Spec.Environments {
		if !envNameRE.MatchString(e.Name) {
			return fmt.Errorf("invalid environment name %q", e.Name)
		}
		if seen[e.Name] {
			return fmt.Errorf("duplicate environment %q", e.Name)
		}
		seen[e.Name] = true
		if err := validateTokenSource(e); err != nil {
			return fmt.Errorf("environment %q: %w", e.Name, err)
		}
	}
	if p.Spec.ReleasesDir == "" {
		p.Spec.ReleasesDir = "releases"
	}
	for i := range p.Spec.Offerings {
		p.Spec.Offerings[i].Category = catalog.NormalizeCategory(p.Spec.Offerings[i].Category)
	}
	return nil
}

func validateTokenSource(e api.Environment) error {
	switch {
	case e.Server == "" && (e.TokenFile != "" || e.CAFile != ""):
		return errors.New("tokenFile and caFile need server")
	case e.Server == "":
		return nil
	case !strings.HasPrefix(e.Server, "https://"):
		return errors.New("server must be an https:// URL")
	case e.TokenFile == "":
		return errors.New("server needs tokenFile")
	case e.InCluster || e.Kubeconfig != "" || e.KubeContext != "" || e.InventoryFile != "" || e.Inventory != "":
		return errors.New("server cannot be combined with inCluster, kubeconfig, kubeContext, inventoryFile or inventory")
	}
	return nil
}

// New loads the platform at path.
func New(path string, opts Options) (*Service, error) {
	p, err := LoadPlatform(path)
	if err != nil {
		return nil, err
	}
	return newService(p, filepath.Dir(path), opts, nil)
}

// NewFromResources creates a service from the namespaced PBOM custom resources.
func NewFromResources(resources Resources, opts Options) (*Service, error) {
	if err := validatePlatform(resources.Platform); err != nil {
		return nil, err
	}
	// Pod files are not part of the configuration, so file references are refused.
	if resources.Platform.Spec.ReadmeFile != "" {
		return nil, errors.New("readmeFile is not supported in a Platform resource; use readme")
	}
	inventories := map[string]*api.Inventory{}
	for _, inv := range resources.Inventories {
		inventories[inv.Metadata.Name] = inv
	}
	for _, e := range resources.Platform.Spec.Environments {
		if e.InventoryFile != "" {
			return nil, fmt.Errorf("environment %q: inventoryFile is not supported in a Platform resource; use inventory", e.Name)
		}
	}
	for i := range resources.Releases {
		r := &resources.Releases[i]
		if r.Kind != api.KindRelease {
			return nil, fmt.Errorf("release %q: expected kind %s, got %q", r.Metadata.Name, api.KindRelease, r.Kind)
		}
		if r.Metadata.Name == "" {
			return nil, errors.New("release metadata.name is required")
		}
		if r.Spec.Version == "" {
			r.Spec.Version = r.Metadata.Name
		}
	}
	release.Sort(resources.Releases)
	s, err := newService(resources.Platform, "", opts, resources.Components)
	if err != nil {
		return nil, err
	}
	s.resourcesReleases = resources.Releases
	s.useResourceReleases = true
	s.inventories = inventories
	return s, nil
}

func newService(p *api.Platform, baseDir string, opts Options, components []*api.Component) (*Service, error) {
	if opts.ReleasesDir != "" {
		p.Spec.ReleasesDir = opts.ReleasesDir
	}
	if opts.ComponentsDir != "" {
		p.Spec.ComponentsDir = opts.ComponentsDir
	}
	s := &Service{Platform: p, baseDir: baseDir, ttl: opts.CacheTTL}
	if s.ttl == 0 {
		s.ttl = 5 * time.Minute
	}
	catalogData, err := catalog.LoadWithComponents(s.resolve(p.Spec.ComponentsDir), components)
	if err != nil {
		return nil, err
	}
	s.Catalog = catalogData
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
func (s *Service) Releases() ([]api.PlatformRelease, error) {
	if s.useResourceReleases {
		return append([]api.PlatformRelease(nil), s.resourcesReleases...), nil
	}
	return release.Load(s.ReleasesDir())
}

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
	if env.Inventory != "" {
		inv, ok := s.inventories[env.Inventory]
		if !ok {
			return nil, fmt.Errorf("inventory %q not found", env.Inventory)
		}
		return forEnvironment(inv, env.Name), nil
	}
	var cfg *rest.Config
	var err error
	contextName := env.KubeContext
	switch {
	case env.InCluster:
		cfg, err = discovery.InClusterConfig()
		contextName = "in-cluster"
	case env.Server != "":
		cfg, err = discovery.TokenConfig(env.Server, s.resolve(env.TokenFile), s.resolve(env.CAFile))
		contextName = env.Server
	default:
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
	return forEnvironment(inv, env), nil
}

// forEnvironment returns a copy of an exported inventory attributed to env.
func forEnvironment(src *api.Inventory, env string) *api.Inventory {
	inv := *src
	inv.Environment = env
	inv.Components = append([]api.DiscoveredComponent(nil), src.Components...)
	for i := range inv.Components {
		inv.Components[i].Category = catalog.NormalizeCategory(inv.Components[i].Category)
	}
	return &inv
}

// CachedInventories returns the last discovered inventories whatever their
// age, discovering only when nothing has been discovered yet.
func (s *Service) CachedInventories(ctx context.Context) map[string]*api.Inventory {
	s.mu.RLock()
	invs := s.invs
	s.mu.RUnlock()
	if invs != nil {
		return invs
	}
	return s.Inventories(ctx, false)
}

// CachedInput is the analysis input from the last discovery and upstream
// fetch, without starting either; ok is false before the first discovery.
func (s *Service) CachedInput() (analysis.Input, bool) {
	rels, err := s.Releases()
	if err != nil {
		return analysis.Input{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.invs == nil {
		return analysis.Input{}, false
	}
	return analysis.Input{
		Platform: s.Platform, Catalog: s.Catalog, Releases: rels,
		Inventories: s.invs, Upstream: s.ups, UpstreamErrors: s.upsErrs,
	}, true
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
			start := time.Now()
			inv, err := s.DiscoverEnvironment(ctx, env)
			metrics.ObserveDiscovery(env.Name, time.Since(start), err)
			if err != nil {
				slog.Warn("discovery failed", "environment", env.Name, "error", err)
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
