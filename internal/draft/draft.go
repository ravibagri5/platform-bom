// Package draft writes a starting description and offerings for a platform
// from what discovery found, for platforms that do not define their own yet.
package draft

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
)

type pillar struct{ title, summary string }

// pillars describe the offering drafted for each category.
var pillars = map[string]pillar{
	"runtime":              {"Kubernetes workloads", "Run containers on Kubernetes with namespaces, quotas and access set up for each team."},
	"infrastructure":       {"Compute and infrastructure", "Nodes, scaling and cloud resources provisioned for workloads on demand."},
	"delivery":             {"Application delivery", "Build and ship applications to every environment through one automated path."},
	"networking":           {"Networking and traffic", "Expose services, route traffic and secure the connections between them."},
	"security":             {"Security and identity", "Certificates, secrets, identity and policy applied to every workload."},
	"observability":        {"Observability", "Metrics, logs, traces and alerts for every workload without extra setup."},
	"data":                 {"Data and storage", "Databases, volumes and backups that teams can request themselves."},
	"streaming":            {"Streaming and messaging", "Event streams and message queues for asynchronous communication."},
	"developer-experience": {"Developer portal", "Templates, docs and tools that make the platform easy to use."},
}

// Title is the drafted offering name for a category.
func Title(category string) string {
	if p, ok := pillars[category]; ok {
		return p.title
	}
	words := strings.FieldsFunc(category, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// Offerings drafts one offering per category of the recognised top-level
// components across invs, in category order. It is empty when nothing was recognised.
func Offerings(invs []*api.Inventory) []api.Offering {
	byCat := map[string]map[string]string{}
	for _, inv := range invs {
		for _, c := range inv.Components {
			if !c.Known || c.PartOf != "" {
				continue
			}
			switch c.Category {
			case "", "other", "platform-api":
				continue
			}
			if byCat[c.Category] == nil {
				byCat[c.Category] = map[string]string{}
			}
			label := c.DisplayName
			if label == "" {
				label = c.Name
			}
			byCat[c.Category][c.Name] = label
		}
	}
	cats := make([]string, 0, len(byCat))
	for cat := range byCat {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		ri, rj := catalog.CategoryRank(cats[i]), catalog.CategoryRank(cats[j])
		if ri != rj {
			return ri < rj
		}
		return cats[i] < cats[j]
	})
	out := make([]api.Offering, 0, len(cats))
	for _, cat := range cats {
		names := make([]string, 0, len(byCat[cat]))
		for n := range byCat[cat] {
			names = append(names, n)
		}
		sort.Strings(names)
		labels := make([]string, len(names))
		for i, n := range names {
			labels[i] = byCat[cat][n]
		}
		desc := "Provided by " + join(labels) + "."
		if p, ok := pillars[cat]; ok {
			desc = p.summary + " " + desc
		}
		out = append(out, api.Offering{
			Name: cat, DisplayName: Title(cat), Category: cat, Status: "ga",
			Description: desc, Components: names,
		})
	}
	return out
}

// Description drafts a one-paragraph platform description, or "" when
// discovery found nothing to describe.
func Description(invs []*api.Inventory, offerings []api.Offering) string {
	k8s := ""
	envs := 0
	for _, inv := range invs {
		if len(inv.Components) == 0 {
			continue
		}
		envs++
		if k8s == "" && inv.Cluster.Version != "" {
			k8s = " " + inv.Cluster.Version
		}
	}
	if envs == 0 {
		return ""
	}
	where := "one cluster"
	if envs > 1 {
		where = fmt.Sprintf("%d environments", envs)
	}
	if len(offerings) == 0 {
		return fmt.Sprintf("A Kubernetes%s platform running in %s.", k8s, where)
	}
	titles := make([]string, len(offerings))
	for i, o := range offerings {
		titles[i] = o.DisplayName
	}
	return fmt.Sprintf("A Kubernetes%s platform running in %s. It gives application teams: %s.", k8s, where, strings.Join(titles, "; "))
}

func join(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
