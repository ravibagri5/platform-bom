// Package landscape classifies tools the catalog does not know using the CNCF
// landscape. A snapshot is embedded, so classification works without network
// access; Refresh replaces it with the live landscape when that is reachable.
package landscape

//go:generate go run ./gen

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
)

// DefaultURL is the CNCF landscape data file.
const DefaultURL = "https://raw.githubusercontent.com/cncf/landscape/master/landscape.yml"

// SourceLabel marks components derived from the landscape.
const SourceLabel = "pbom.dev/source"

// minProjects rejects a download whose format changed so much that few projects parse.
const minProjects = 100

const maxDownload = 32 << 20

// Project is a CNCF project, classified into a platform pillar.
type Project struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Maturity    string   `json:"maturity"`
	Description string   `json:"description,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	GitHub      string   `json:"github,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

//go:embed cncf.json
var snapshot []byte

type index struct {
	byKey  map[string]*Project
	source string
}

var current atomic.Pointer[index]

func init() {
	var projects []Project
	if err := json.Unmarshal(snapshot, &projects); err != nil {
		panic(fmt.Sprintf("embedded CNCF landscape snapshot: %v", err))
	}
	current.Store(newIndex(projects, "snapshot"))
}

func newIndex(projects []Project, source string) *index {
	idx := &index{byKey: map[string]*Project{}, source: source}
	for i := range projects {
		p := &projects[i]
		for _, k := range append([]string{p.Key}, p.Aliases...) {
			if _, taken := idx.byKey[k]; !taken {
				idx.byKey[k] = p
			}
		}
	}
	return idx
}

// Source reports where the current data came from: "snapshot" or a URL.
func Source() string { return current.Load().source }

// Count is the number of projects currently known.
func Count() int {
	seen := map[*Project]bool{}
	for _, p := range current.Load().byKey {
		seen[p] = true
	}
	return len(seen)
}

// chartSuffixes are stripped from Helm chart names before matching.
var chartSuffixes = []string{"-operator", "-controller", "-crds", "-chart", "-helm"}

// Match returns the project a Helm chart name refers to, or nil.
func Match(chart string) *Project {
	idx := current.Load()
	name := strings.ToLower(chart)
	if p := idx.byKey[name]; p != nil {
		return p
	}
	for _, s := range chartSuffixes {
		if base, ok := strings.CutSuffix(name, s); ok && len(base) >= 3 {
			if p := idx.byKey[base]; p != nil {
				return p
			}
		}
	}
	return nil
}

// Get returns the project with the given key or alias, or nil.
func Get(key string) *Project { return current.Load().byKey[key] }

// Component turns a project into a catalog Component.
func Component(p *Project) *api.Component {
	desc := p.Description
	if desc == "" {
		desc = "CNCF " + p.Maturity + " project."
	}
	c := &api.Component{
		TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindComponent},
		Metadata: api.Metadata{Name: p.Key, Labels: map[string]string{SourceLabel: "cncf-landscape"}},
		Spec: api.ComponentSpec{
			DisplayName: p.Name,
			Category:    p.Category,
			Description: desc,
			Homepage:    p.Homepage,
			Discovery:   api.Discovery{HelmCharts: append([]string{p.Key}, p.Aliases...)},
		},
	}
	if p.GitHub != "" {
		c.Spec.Upstream = &api.Upstream{GitHub: p.GitHub}
	}
	return c
}

// Refresh replaces the snapshot with the landscape at url. On any failure the
// current data is kept, so callers can log the error and carry on.
func Refresh(ctx context.Context, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	data, err := download(ctx, url)
	if err != nil {
		return 0, err
	}
	projects, err := Parse(data)
	if err != nil {
		return 0, err
	}
	current.Store(newIndex(projects, url))
	return len(projects), nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

type landscapeFile struct {
	Landscape []struct {
		Name          string `json:"name"`
		Subcategories []struct {
			Name  string `json:"name"`
			Items []struct {
				Name        string `json:"name"`
				Homepage    string `json:"homepage_url"`
				Repo        string `json:"repo_url"`
				Project     string `json:"project"`
				Description string `json:"description"`
			} `json:"items"`
		} `json:"subcategories"`
	} `json:"landscape"`
}

var (
	nonSlug  = regexp.MustCompile(`[^a-z0-9]+`)
	githubRE = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?/?$`)
)

// Parse reads a CNCF landscape.yml and returns its graduated, incubating and
// sandbox projects sorted by key.
func Parse(data []byte) ([]Project, error) {
	var f landscapeFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse landscape: %w", err)
	}
	byKey := map[string]Project{}
	for _, cat := range f.Landscape {
		for _, sub := range cat.Subcategories {
			for _, it := range sub.Items {
				switch it.Project {
				case "graduated", "incubating", "sandbox":
				default:
					continue
				}
				key := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(it.Name), "-"), "-")
				if len(key) < 3 {
					continue
				}
				if _, dup := byKey[key]; dup {
					continue
				}
				p := Project{
					Key: key, Name: it.Name, Category: pillar(cat.Name, sub.Name), Maturity: it.Project,
					Description: strings.TrimSpace(it.Description), Homepage: it.Homepage,
				}
				if m := githubRE.FindStringSubmatch(it.Repo); m != nil {
					p.GitHub = m[1]
					if alias := strings.ToLower(m[1][strings.Index(m[1], "/")+1:]); alias != key && len(alias) >= 3 {
						p.Aliases = []string{alias}
					}
				}
				byKey[key] = p
			}
		}
	}
	if len(byKey) < minProjects {
		return nil, fmt.Errorf("landscape has %d CNCF projects, expected at least %d; format may have changed", len(byKey), minProjects)
	}
	out := make([]Project, 0, len(byKey))
	for _, p := range byKey {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// subcategoryPillars maps landscape subcategories to PBOM categories.
var subcategoryPillars = map[string]string{
	"Automation & Configuration":           "infrastructure",
	"Container Registry":                   "delivery",
	"Security & Compliance":                "security",
	"Key Management":                       "security",
	"Cloud Native Storage":                 "data",
	"Container Runtime":                    "runtime",
	"Cloud Native Network":                 "networking",
	"Scheduling & Orchestration":           "runtime",
	"Coordination & Service Discovery":     "networking",
	"Remote Procedure Call":                "networking",
	"Service Proxy":                        "networking",
	"API Gateway":                          "networking",
	"Service Mesh":                         "networking",
	"Database":                             "data",
	"Streaming & Messaging":                "streaming",
	"Application Definition & Image Build": "delivery",
	"Continuous Integration & Delivery":    "delivery",
	"Feature Flagging":                     "delivery",
	"Chaos Engineering":                    "observability",
	"Continuous Optimization":              "observability",
	"Observability":                        "observability",
	"Governance, Policy and Security":      "security",
	"Storage":                              "data",
	"Network":                              "networking",
	"Gateway":                              "networking",
	"Continuous Integration and Delivery":  "delivery",
	"Data Architecture":                    "data",
	"Data Science":                         "data",
}

// categoryPillars is the fallback for subcategories missing above.
var categoryPillars = map[string]string{
	"Provisioning":                   "infrastructure",
	"Runtime":                        "runtime",
	"Orchestration & Management":     "networking",
	"App Definition and Development": "delivery",
	"Platform":                       "runtime",
	"Serverless":                     "runtime",
	"Observability and Analysis":     "observability",
	"Wasm":                           "runtime",
}

func pillar(category, subcategory string) string {
	if p, ok := subcategoryPillars[subcategory]; ok {
		return p
	}
	if p, ok := categoryPillars[category]; ok {
		return p
	}
	return "other"
}
