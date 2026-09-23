// Package analysis turns inventories, releases and upstream data into the
// platform views: the environment matrix, drift and update recommendations.
package analysis

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/upstream"
	"github.com/ravibagri5/platform-bom/internal/version"
)

// Freshness describes how far a version is behind upstream.
type Freshness string

// Freshness values, ordered from best to worst.
const (
	FreshUnknown     Freshness = "unknown"
	FreshLatest      Freshness = "latest"
	FreshPatch       Freshness = "patch-available"
	FreshMinor       Freshness = "minor-behind"
	FreshMajor       Freshness = "major-behind"
	FreshUnsupported Freshness = "unsupported"
)

var freshnessRank = map[Freshness]int{
	FreshUnknown: 0, FreshLatest: 1, FreshPatch: 2, FreshMinor: 3, FreshMajor: 4, FreshUnsupported: 5,
}

// Cell statuses relative to an environment's target release.
const (
	StatusAligned   = "aligned"
	StatusDrift     = "drift"
	StatusMissing   = "missing"
	StatusUntracked = "untracked"
	StatusPresent   = "present"
	StatusAbsent    = "absent"
)

// Input is everything the analysis needs.
type Input struct {
	Platform *api.Platform
	Catalog  *catalog.Catalog
	// Releases sorted newest first.
	Releases    []api.PlatformRelease
	Inventories map[string]*api.Inventory
	// Upstream releases per component, newest first.
	Upstream       map[string][]upstream.Release
	UpstreamErrors map[string]string
}

// Matrix is the component × environment view of the platform.
type Matrix struct {
	GeneratedAt    time.Time   `json:"generatedAt"`
	CurrentRelease string      `json:"currentRelease,omitempty"`
	Environments   []EnvStatus `json:"environments"`
	Rows           []Row       `json:"rows"`
	Summary        Summary     `json:"summary"`
}

// EnvStatus summarises one environment.
type EnvStatus struct {
	Name           string          `json:"name"`
	DisplayName    string          `json:"displayName,omitempty"`
	Tier           string          `json:"tier,omitempty"`
	TargetRelease  string          `json:"targetRelease,omitempty"`
	MatchedRelease string          `json:"matchedRelease,omitempty"`
	Compliant      bool            `json:"compliant"`
	Cluster        api.ClusterInfo `json:"cluster"`
	CollectedAt    time.Time       `json:"collectedAt"`
	Errors         []string        `json:"errors,omitempty"`
	Aligned        int             `json:"aligned"`
	Drift          int             `json:"drift"`
	Missing        int             `json:"missing"`
	Untracked      int             `json:"untracked"`
}

// Row is one component across environments.
type Row struct {
	Component   string          `json:"component"`
	DisplayName string          `json:"displayName"`
	Category    string          `json:"category"`
	Known       bool            `json:"known"`
	PartOf      string          `json:"partOf,omitempty"`
	Kind        string          `json:"kind,omitempty"`
	Declared    string          `json:"declared,omitempty"`
	Latest      string          `json:"latest,omitempty"`
	Freshness   Freshness       `json:"freshness"`
	Cells       map[string]Cell `json:"cells"`
}

// Cell is one component in one environment.
type Cell struct {
	Version   string    `json:"version,omitempty"`
	Expected  string    `json:"expected,omitempty"`
	Status    string    `json:"status"`
	Freshness Freshness `json:"freshness"`
}

// Summary holds headline numbers. Percentages are -1 when not applicable.
type Summary struct {
	Components   int `json:"components"`
	Environments int `json:"environments"`
	Offerings    int `json:"offerings"`
	Releases     int `json:"releases"`
	Aligned      int `json:"aligned"`
	Drift        int `json:"drift"`
	Missing      int `json:"missing"`
	Untracked    int `json:"untracked"`
	UpToDate     int `json:"upToDate"`
	Behind       int `json:"behind"`
	Unsupported  int `json:"unsupported"`
	Alignment    int `json:"alignment"`
	Currency     int `json:"currency"`
}

// Assess compares current against the newest upstream release.
func Assess(current string, releases []upstream.Release, supportedMinors int) (Freshness, int) {
	cur, ok := version.Parse(current)
	if !ok || len(releases) == 0 {
		return FreshUnknown, 0
	}
	latest, ok := version.Parse(releases[0].Version)
	if !ok {
		return FreshUnknown, 0
	}
	if version.Compare(cur, latest) >= 0 {
		return FreshLatest, 0
	}
	if cur.Major < latest.Major {
		return FreshMajor, 0
	}
	behind := latest.Minor - cur.Minor
	switch {
	case supportedMinors > 0 && behind >= supportedMinors:
		return FreshUnsupported, behind
	case behind > 0:
		return FreshMinor, behind
	}
	return FreshPatch, 0
}

func worse(a, b Freshness) Freshness {
	if freshnessRank[b] > freshnessRank[a] {
		return b
	}
	return a
}

// FindRelease returns the release with the given name.
func FindRelease(releases []api.PlatformRelease, name string) *api.PlatformRelease {
	for i := range releases {
		if releases[i].Metadata.Name == name {
			return &releases[i]
		}
	}
	return nil
}

func versionsOf(inv *api.Inventory) map[string]string {
	out := map[string]string{}
	if inv == nil {
		return out
	}
	for _, c := range inv.Components {
		out[c.Name] = c.Version
	}
	return out
}

// Satisfies reports whether the observed versions fulfil every component of rel.
func Satisfies(rel *api.PlatformRelease, actual map[string]string) bool {
	for name, want := range rel.Spec.Components {
		got, ok := actual[name]
		if !ok || !version.Matches(want, got) {
			return false
		}
	}
	return true
}

func supportedMinors(cat *catalog.Catalog, name string) int {
	if c, ok := cat.Get(name); ok && c.Spec.Upstream != nil {
		return c.Spec.Upstream.SupportedMinors
	}
	return 0
}

func percent(n, total int) int {
	if total == 0 {
		return -1
	}
	return int(math.Round(100 * float64(n) / float64(total)))
}

// BuildMatrix computes the environment matrix.
func BuildMatrix(in Input) *Matrix {
	m := &Matrix{GeneratedAt: time.Now().UTC()}
	var current *api.PlatformRelease
	if len(in.Releases) > 0 {
		current = &in.Releases[0]
		m.CurrentRelease = current.Metadata.Name
	}
	rows := map[string]*Row{}
	row := func(name string) *Row {
		if r, ok := rows[name]; ok {
			return r
		}
		r := &Row{Component: name, DisplayName: name, Category: "other", Cells: map[string]Cell{}, Freshness: FreshUnknown}
		if c, ok := in.Catalog.Get(name); ok {
			r.DisplayName, r.Category, r.Known, r.PartOf = c.Spec.DisplayName, c.Spec.Category, true, c.Spec.PartOf
		}
		if rels := in.Upstream[name]; len(rels) > 0 {
			r.Latest = rels[0].Version
		}
		rows[name] = r
		return r
	}
	if current != nil {
		for name, v := range current.Spec.Components {
			row(name).Declared = v
		}
	}
	for _, env := range in.Platform.Spec.Environments {
		if inv := in.Inventories[env.Name]; inv != nil {
			for _, c := range inv.Components {
				r := row(c.Name)
				if r.Kind == "" {
					r.Kind = c.Kind
				}
				if !r.Known && c.DisplayName != "" {
					r.DisplayName, r.Category, r.PartOf = c.DisplayName, c.Category, c.PartOf
				}
			}
		}
		if target := FindRelease(in.Releases, env.TargetRelease); target != nil {
			for name := range target.Spec.Components {
				row(name)
			}
		}
	}

	for _, env := range in.Platform.Spec.Environments {
		inv := in.Inventories[env.Name]
		actual := versionsOf(inv)
		es := EnvStatus{Name: env.Name, DisplayName: env.DisplayName, Tier: env.Tier, TargetRelease: env.TargetRelease}
		if inv != nil {
			es.Cluster, es.CollectedAt, es.Errors = inv.Cluster, inv.CollectedAt, inv.Errors
		}
		target := FindRelease(in.Releases, env.TargetRelease)
		expected := map[string]string{}
		if target != nil {
			expected = target.Spec.Components
		}
		for i := range in.Releases {
			if inv != nil && Satisfies(&in.Releases[i], actual) {
				es.MatchedRelease = in.Releases[i].Metadata.Name
				break
			}
		}
		for name, r := range rows {
			got, found := actual[name]
			want, declared := expected[name]
			cell := Cell{Version: got, Expected: want, Freshness: FreshUnknown}
			switch {
			case target == nil && found:
				cell.Status = StatusPresent
			case target == nil:
				cell.Status = StatusAbsent
			case declared && !found:
				cell.Status = StatusMissing
				es.Missing++
			case declared && version.Matches(want, got):
				cell.Status = StatusAligned
				es.Aligned++
			case declared:
				cell.Status = StatusDrift
				es.Drift++
			// Version-less components (seen only via API groups) cannot be pinned in a release.
			case found && got == "":
				cell.Status = StatusPresent
			case found:
				cell.Status = StatusUntracked
				es.Untracked++
			default:
				cell.Status = StatusAbsent
			}
			if found {
				cell.Freshness, _ = Assess(got, in.Upstream[name], supportedMinors(in.Catalog, name))
				r.Freshness = worse(r.Freshness, cell.Freshness)
			}
			r.Cells[env.Name] = cell
		}
		es.Compliant = target != nil && es.Drift == 0 && es.Missing == 0
		m.Environments = append(m.Environments, es)
	}

	for _, r := range rows {
		m.Rows = append(m.Rows, *r)
	}
	sort.Slice(m.Rows, func(i, j int) bool {
		a, b := m.Rows[i], m.Rows[j]
		if ra, rb := catalog.CategoryRank(a.Category), catalog.CategoryRank(b.Category); ra != rb {
			return ra < rb
		}
		return strings.ToLower(a.DisplayName) < strings.ToLower(b.DisplayName)
	})

	s := &m.Summary
	s.Components, s.Environments, s.Releases = len(m.Rows), len(m.Environments), len(in.Releases)
	s.Offerings = len(in.Platform.Spec.Offerings)
	for _, es := range m.Environments {
		s.Aligned += es.Aligned
		s.Drift += es.Drift
		s.Missing += es.Missing
		s.Untracked += es.Untracked
	}
	assessed := 0
	for _, r := range m.Rows {
		switch r.Freshness {
		case FreshUnknown:
			continue
		case FreshLatest:
			s.UpToDate++
		case FreshUnsupported:
			s.Unsupported++
			s.Behind++
		default:
			s.Behind++
		}
		assessed++
	}
	s.Alignment = percent(s.Aligned, s.Aligned+s.Drift+s.Missing)
	s.Currency = percent(s.UpToDate, assessed)
	return m
}
