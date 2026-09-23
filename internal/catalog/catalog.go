// Package catalog holds Component definitions: the community knowledge base
// that turns raw cluster evidence into named, versioned platform components.
package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
)

//go:embed components/*.yaml
var builtin embed.FS

var docSeparator = regexp.MustCompile(`(?m)^---\s*$`)

// Categories in display order.
var Categories = []string{
	"runtime", "compute", "delivery", "infrastructure", "composition", "platform-api",
	"networking", "security", "observability", "data", "other",
}

// CategoryRank returns the display order of a category.
func CategoryRank(c string) int {
	for i, v := range Categories {
		if v == c {
			return i
		}
	}
	return len(Categories)
}

// Catalog is an indexed set of Component definitions.
type Catalog struct {
	byName map[string]*api.Component
	names  []string
}

// Load returns the builtin catalog merged with definitions from extraDir.
// Definitions in extraDir override builtin ones with the same name.
func Load(extraDir string) (*Catalog, error) {
	c := &Catalog{byName: map[string]*api.Component{}}
	if err := c.loadFS(builtin, "components"); err != nil {
		return nil, fmt.Errorf("builtin catalog: %w", err)
	}
	if extraDir != "" {
		if err := c.loadFS(os.DirFS(extraDir), "."); err != nil {
			return nil, fmt.Errorf("catalog %s: %w", extraDir, err)
		}
	}
	for n := range c.byName {
		c.names = append(c.names, n)
	}
	sort.Strings(c.names)
	return c, nil
}

func (c *Catalog) loadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		comps, err := Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		for _, comp := range comps {
			c.byName[comp.Metadata.Name] = comp
		}
	}
	return nil
}

// Parse decodes one or more YAML Component documents.
func Parse(data []byte) ([]*api.Component, error) {
	docs := docSeparator.Split(string(data), -1)
	out := make([]*api.Component, 0, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		comp := &api.Component{}
		if err := yaml.UnmarshalStrict([]byte(doc), comp); err != nil {
			return nil, err
		}
		if comp.Kind != api.KindComponent {
			return nil, fmt.Errorf("unexpected kind %q", comp.Kind)
		}
		if comp.Metadata.Name == "" {
			return nil, fmt.Errorf("component without metadata.name")
		}
		if comp.Spec.DisplayName == "" {
			comp.Spec.DisplayName = comp.Metadata.Name
		}
		if comp.Spec.Category == "" {
			comp.Spec.Category = "other"
		}
		out = append(out, comp)
	}
	return out, nil
}

// List returns all components sorted by name.
func (c *Catalog) List() []*api.Component {
	out := make([]*api.Component, 0, len(c.names))
	for _, n := range c.names {
		out = append(out, c.byName[n])
	}
	return out
}

// Get returns a component by name.
func (c *Catalog) Get(name string) (*api.Component, bool) {
	comp, ok := c.byName[name]
	return comp, ok
}

// Kubernetes returns the component that represents the cluster itself.
func (c *Catalog) Kubernetes() *api.Component {
	for _, n := range c.names {
		if c.byName[n].Spec.Discovery.Kubernetes {
			return c.byName[n]
		}
	}
	return nil
}

// MatchImage returns the component whose image patterns match repo.
func (c *Catalog) MatchImage(repo string) *api.Component {
	return c.match(repo, func(d api.Discovery) []string { return d.Images })
}

// MatchHelmChart returns the component whose chart patterns match chart.
func (c *Catalog) MatchHelmChart(chart string) *api.Component {
	return c.match(chart, func(d api.Discovery) []string { return d.HelmCharts })
}

// MatchCrossplanePackage returns the component whose package patterns match repo.
func (c *Catalog) MatchCrossplanePackage(repo string) *api.Component {
	return c.match(repo, func(d api.Discovery) []string { return d.CrossplanePackages })
}

// MatchAPIGroup returns all components that declare group.
func (c *Catalog) MatchAPIGroup(group string) []*api.Component {
	var out []*api.Component
	for _, n := range c.names {
		for _, g := range c.byName[n].Spec.Discovery.APIGroups {
			if g == group {
				out = append(out, c.byName[n])
				break
			}
		}
	}
	return out
}

func (c *Catalog) match(ref string, patterns func(api.Discovery) []string) *api.Component {
	for _, n := range c.names {
		comp := c.byName[n]
		for _, p := range patterns(comp.Spec.Discovery) {
			if MatchRef(p, ref) {
				return comp
			}
		}
	}
	return nil
}

// MatchRef matches a glob pattern against ref or any trailing path of ref, so
// "argoproj/argocd" matches "quay.io/argoproj/argocd" and mirrored copies such
// as "registry.corp/mirror/argoproj/argocd".
func MatchRef(pattern, ref string) bool {
	segs := strings.Split(ref, "/")
	for i := range segs {
		if ok, _ := path.Match(pattern, strings.Join(segs[i:], "/")); ok {
			return true
		}
	}
	return false
}

// SplitImage splits an image reference into repository and tag, dropping any digest.
func SplitImage(ref string) (repo, tag string) {
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	slash := strings.LastIndex(ref, "/")
	if colon := strings.LastIndex(ref, ":"); colon > slash {
		return ref[:colon], ref[colon+1:]
	}
	return ref, ""
}
