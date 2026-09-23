// Package release manages PlatformRelease documents: loading, cutting new
// releases from live inventory and diffing releases.
package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/version"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// ValidName reports whether name is safe to use as a release file name.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// Load reads all releases in dir, newest first. A missing dir yields no releases.
func Load(dir string) ([]api.PlatformRelease, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]api.PlatformRelease, 0, len(entries))
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r api.PlatformRelease
		if err := yaml.UnmarshalStrict(data, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if r.Kind != api.KindRelease {
			return nil, fmt.Errorf("%s: unexpected kind %q", e.Name(), r.Kind)
		}
		if r.Metadata.Name == "" {
			r.Metadata.Name = e.Name()[:len(e.Name())-len(ext)]
		}
		if r.Spec.Version == "" {
			r.Spec.Version = r.Metadata.Name
		}
		out = append(out, r)
	}
	Sort(out)
	return out, nil
}

// Sort orders releases newest first by version.
func Sort(rels []api.PlatformRelease) {
	sort.SliceStable(rels, func(i, j int) bool {
		return version.CompareStrings(rels[i].Spec.Version, rels[j].Spec.Version) > 0
	})
}

// FromInventory cuts a release from the components observed in inv.
func FromInventory(name string, inv *api.Inventory, offerings []api.Offering, summary string) *api.PlatformRelease {
	r := &api.PlatformRelease{
		TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindRelease},
		Metadata: api.Metadata{Name: name},
		Spec: api.ReleaseSpec{
			Version:    version.Normalize(name),
			Date:       time.Now().UTC().Format("2006-01-02"),
			Summary:    summary,
			Components: map[string]string{},
		},
	}
	for _, c := range inv.Components {
		if c.Version != "" {
			r.Spec.Components[c.Name] = c.Version
		}
	}
	for _, o := range offerings {
		if o.Status == "planned" {
			continue
		}
		r.Spec.Offerings = append(r.Spec.Offerings, o.Name)
	}
	return r
}

// Write stores r in dir as <name>.yaml, refusing to overwrite unless force is set.
func Write(dir string, r *api.PlatformRelease, force bool) (string, error) {
	if !ValidName(r.Metadata.Name) {
		return "", fmt.Errorf("invalid release name %q", r.Metadata.Name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, r.Metadata.Name+".yaml")
	if _, err := os.Stat(p); err == nil && !force {
		return "", fmt.Errorf("release %s already exists (use --force to overwrite)", p)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return "", err
	}
	return p, os.WriteFile(p, data, 0o644)
}

// Change kinds.
const (
	ChangeAdded     = "added"
	ChangeRemoved   = "removed"
	ChangeUpgraded  = "upgraded"
	ChangeDowngrade = "downgraded"
	ChangeUnchanged = "unchanged"
)

// ComponentChange is one component's difference between two releases.
type ComponentChange struct {
	Component   string `json:"component"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Change      string `json:"change"`
}

// Diff is the difference between two releases.
type Diff struct {
	From             string            `json:"from"`
	To               string            `json:"to"`
	Components       []ComponentChange `json:"components"`
	OfferingsAdded   []string          `json:"offeringsAdded,omitempty"`
	OfferingsRemoved []string          `json:"offeringsRemoved,omitempty"`
}

// Compare diffs two releases.
func Compare(from, to *api.PlatformRelease, cat *catalog.Catalog) Diff {
	d := Diff{From: from.Metadata.Name, To: to.Metadata.Name}
	names := map[string]bool{}
	for n := range from.Spec.Components {
		names[n] = true
	}
	for n := range to.Spec.Components {
		names[n] = true
	}
	for n := range names {
		a, inA := from.Spec.Components[n]
		b, inB := to.Spec.Components[n]
		ch := ComponentChange{Component: n, DisplayName: n, Category: "other", From: a, To: b}
		if c, ok := cat.Get(n); ok {
			ch.DisplayName, ch.Category = c.Spec.DisplayName, c.Spec.Category
		}
		switch cmp := version.CompareStrings(a, b); {
		case !inA:
			ch.Change = ChangeAdded
		case !inB:
			ch.Change = ChangeRemoved
		case cmp < 0:
			ch.Change = ChangeUpgraded
		case cmp > 0:
			ch.Change = ChangeDowngrade
		default:
			ch.Change = ChangeUnchanged
		}
		d.Components = append(d.Components, ch)
	}
	sort.Slice(d.Components, func(i, j int) bool {
		a, b := d.Components[i], d.Components[j]
		if ra, rb := catalog.CategoryRank(a.Category), catalog.CategoryRank(b.Category); ra != rb {
			return ra < rb
		}
		return strings.ToLower(a.DisplayName) < strings.ToLower(b.DisplayName)
	})
	d.OfferingsAdded = minus(to.Spec.Offerings, from.Spec.Offerings)
	d.OfferingsRemoved = minus(from.Spec.Offerings, to.Spec.Offerings)
	return d
}

func minus(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}
