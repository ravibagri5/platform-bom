package draft

import (
	"strings"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/release"
)

// Label marks a drafted PlatformRelease.
const Label = "pbom.dev/drafted"

// FirstRelease is the name of a drafted first release.
const FirstRelease = "1.0.0"

// Apply fills what the platform leaves empty from what discovery found and
// returns the names of the drafted parts: description, offerings, release
// (a first release from what one environment runs now) and targets
// (environments without targetRelease compared with the newest release).
// p and rels are not modified.
func Apply(p *api.Platform, rels []api.PlatformRelease, byEnv map[string]*api.Inventory) (*api.Platform, []api.PlatformRelease, []string) {
	invs := make([]*api.Inventory, 0, len(byEnv))
	for _, e := range p.Spec.Environments {
		if inv := byEnv[e.Name]; inv != nil {
			invs = append(invs, inv)
		}
	}
	out := *p
	var drafted []string
	if len(out.Spec.Offerings) == 0 {
		if offs := Offerings(invs); len(offs) > 0 {
			out.Spec.Offerings = offs
			drafted = append(drafted, "offerings")
		}
	}
	if out.Spec.Description == "" {
		if desc := Description(invs, out.Spec.Offerings); desc != "" {
			out.Spec.Description = desc
			drafted = append(drafted, "description")
		}
	}
	if len(rels) == 0 {
		if env := SourceEnvironment(&out, byEnv); env != "" {
			rel := Release(FirstRelease, env, byEnv[env], out.Spec.Offerings)
			rel.Metadata.Labels = map[string]string{Label: "true"}
			rels = []api.PlatformRelease{*rel}
			drafted = append(drafted, "release")
		}
	}
	if len(rels) > 0 {
		envs := make([]api.Environment, len(out.Spec.Environments))
		copy(envs, out.Spec.Environments)
		targeted := false
		for i := range envs {
			if envs[i].TargetRelease == "" {
				envs[i].TargetRelease = rels[0].Metadata.Name
				targeted = true
			}
		}
		if targeted {
			out.Spec.Environments = envs
			drafted = append(drafted, "targets")
		}
	}
	return &out, rels, drafted
}

// SourceEnvironment picks the environment a first release is drafted from:
// a production one when there is one, otherwise the first that found anything.
func SourceEnvironment(p *api.Platform, byEnv map[string]*api.Inventory) string {
	first := ""
	for _, e := range p.Spec.Environments {
		inv := byEnv[e.Name]
		if inv == nil || len(inv.Components) == 0 {
			continue
		}
		if e.Tier == "production" || strings.Contains(e.Name, "prod") {
			return e.Name
		}
		if first == "" {
			first = e.Name
		}
	}
	return first
}

// Release snapshots what env runs now as a release named name.
func Release(name, env string, inv *api.Inventory, offerings []api.Offering) *api.PlatformRelease {
	return release.FromInventory(name, inv, offerings, "What "+env+" runs now")
}
