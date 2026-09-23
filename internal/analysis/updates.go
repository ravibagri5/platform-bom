package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ravibagri5/platform-bom/internal/upstream"
	"github.com/ravibagri5/platform-bom/internal/version"
)

const maxNewer = 15

// Update is an explainable upgrade recommendation for one component.
type Update struct {
	Component   string            `json:"component"`
	DisplayName string            `json:"displayName"`
	Category    string            `json:"category"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Versions    map[string]string `json:"versions"`
	// Current is the oldest version running in any environment.
	Current        string             `json:"current,omitempty"`
	CurrentRelease *upstream.Release  `json:"currentRelease,omitempty"`
	Latest         *upstream.Release  `json:"latest,omitempty"`
	Freshness      Freshness          `json:"freshness"`
	MinorsBehind   int                `json:"minorsBehind"`
	Reasons        []string           `json:"reasons"`
	Newer          []upstream.Release `json:"newer,omitempty"`
	Error          string             `json:"error,omitempty"`
}

// BuildUpdates returns recommendations for every discovered component,
// most urgent first.
func BuildUpdates(in Input) []Update {
	versions := map[string]map[string]string{}
	for _, env := range in.Platform.Spec.Environments {
		inv := in.Inventories[env.Name]
		if inv == nil {
			continue
		}
		for _, c := range inv.Components {
			if versions[c.Name] == nil {
				versions[c.Name] = map[string]string{}
			}
			if c.Version != "" {
				versions[c.Name][env.Name] = c.Version
			}
		}
	}

	out := make([]Update, 0, len(versions))
	for name, perEnv := range versions {
		u := Update{Component: name, DisplayName: name, Category: "other", Versions: perEnv, Freshness: FreshUnknown}
		if c, ok := in.Catalog.Get(name); ok {
			u.DisplayName, u.Category, u.Homepage = c.Spec.DisplayName, c.Spec.Category, c.Spec.Homepage
			if c.Spec.Upstream != nil && c.Spec.Upstream.GitHub != "" {
				u.Repository = "https://github.com/" + c.Spec.Upstream.GitHub
			}
		}
		u.Current = oldest(perEnv)
		u.Error = in.UpstreamErrors[name]
		rels := in.Upstream[name]
		if len(rels) > 0 {
			u.Latest = &rels[0]
		}
		u.Freshness, u.MinorsBehind = Assess(u.Current, rels, supportedMinors(in.Catalog, name))
		u.CurrentRelease = findVersion(rels, u.Current)
		u.Newer = newerThan(rels, u.Current)
		u.Reasons = reasons(u, rels, supportedMinors(in.Catalog, name))
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		if ri, rj := freshnessRank[out[i].Freshness], freshnessRank[out[j].Freshness]; ri != rj {
			return ri > rj
		}
		if out[i].MinorsBehind != out[j].MinorsBehind {
			return out[i].MinorsBehind > out[j].MinorsBehind
		}
		return out[i].DisplayName < out[j].DisplayName
	})
	return out
}

func oldest(perEnv map[string]string) string {
	var min string
	for _, v := range perEnv {
		if min == "" || version.CompareStrings(v, min) < 0 {
			min = v
		}
	}
	return min
}

func findVersion(rels []upstream.Release, v string) *upstream.Release {
	for i := range rels {
		if version.CompareStrings(rels[i].Version, v) == 0 {
			return &rels[i]
		}
	}
	return nil
}

func newerThan(rels []upstream.Release, current string) []upstream.Release {
	if _, ok := version.Parse(current); !ok {
		return nil
	}
	out := make([]upstream.Release, 0, maxNewer)
	for _, r := range rels {
		if version.CompareStrings(r.Version, current) <= 0 || len(out) == maxNewer {
			break
		}
		out = append(out, r)
	}
	return out
}

func latestPatch(rels []upstream.Release, current string) *upstream.Release {
	cur, ok := version.Parse(current)
	if !ok {
		return nil
	}
	for i := range rels {
		v, ok := version.Parse(rels[i].Version)
		if ok && v.Major == cur.Major && v.Minor == cur.Minor && version.Compare(v, cur) > 0 {
			return &rels[i]
		}
	}
	return nil
}

func day(t time.Time) string { return t.Format("2 Jan 2006") }

func reasons(u Update, rels []upstream.Release, supported int) []string {
	var r []string
	switch {
	case u.Error != "" && len(rels) == 0:
		r = append(r, "Upstream release data unavailable: "+u.Error)
	case u.Latest == nil:
		r = append(r, "No upstream source configured for this component.")
	}
	if u.Latest != nil {
		switch u.Freshness {
		case FreshLatest:
			r = append(r, fmt.Sprintf("Running the latest upstream release %s.", u.Latest.Version))
		case FreshPatch:
			r = append(r, fmt.Sprintf("Patch release %s is available for the %s line (bug and security fixes).",
				u.Latest.Version, version.Normalize(u.Current)))
		case FreshMinor:
			r = append(r, fmt.Sprintf("%d minor release(s) behind latest %s.", u.MinorsBehind, u.Latest.Version))
		case FreshMajor:
			r = append(r, fmt.Sprintf("New major version %s is available; review breaking changes before upgrading.", u.Latest.Version))
		case FreshUnsupported:
			r = append(r, fmt.Sprintf("Upstream supports the latest %d minor releases; %s is %d minors behind %s and out of upstream support.",
				supported, u.Current, u.MinorsBehind, u.Latest.Version))
		}
		if u.Freshness != FreshLatest && !u.Latest.PublishedAt.IsZero() {
			r = append(r, fmt.Sprintf("%s was released on %s.", u.Latest.Version, day(u.Latest.PublishedAt)))
		}
	}
	if u.Freshness == FreshMinor || u.Freshness == FreshMajor || u.Freshness == FreshUnsupported {
		if p := latestPatch(rels, u.Current); p != nil {
			r = append(r, fmt.Sprintf("Low-risk step: patch %s is available within your current line.", p.Version))
		}
	}
	if u.CurrentRelease != nil && !u.CurrentRelease.PublishedAt.IsZero() {
		age := int(time.Since(u.CurrentRelease.PublishedAt).Hours() / 24)
		r = append(r, fmt.Sprintf("Current version %s was released %d days ago.", u.Current, age))
	}
	if distinct(u.Versions) > 1 {
		parts := make([]string, 0, len(u.Versions))
		for env, v := range u.Versions {
			parts = append(parts, env+" "+v)
		}
		sort.Strings(parts)
		r = append(r, "Environments run different versions: "+strings.Join(parts, ", ")+".")
	}
	return r
}

func distinct(m map[string]string) int {
	seen := map[string]bool{}
	for _, v := range m {
		seen[v] = true
	}
	return len(seen)
}
