// Package discovery builds an Inventory from a live Kubernetes cluster.
package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/version"
)

const pageSize = 500

// Options configure a discovery run.
type Options struct {
	Environment string
	Context     string
	Policy      api.DiscoveryPolicy
}

// RESTConfig loads a client config for kubeconfig and context. Empty values
// use the default loading rules, falling back to in-cluster config.
func RESTConfig(kubeconfig, context string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules,
		&clientcmd.ConfigOverrides{CurrentContext: context}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return tune(cfg), nil
}

// InClusterConfig uses the service account of the pod pbom runs in.
func InClusterConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return tune(cfg), nil
}

func tune(cfg *rest.Config) *rest.Config {
	cfg.QPS, cfg.Burst, cfg.Timeout = 50, 100, 30*time.Second
	return cfg
}

// Discover inspects the cluster and returns the components it runs.
// Failures of individual sources are recorded in Inventory.Errors.
func Discover(ctx context.Context, cfg *rest.Config, cat *catalog.Catalog, opts Options) (*api.Inventory, error) {
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	d := &discoverer{
		kc: kc, dc: dc, cat: cat, opts: opts,
		col:     newCollector(),
		ignored: map[string]bool{},
	}
	for _, ns := range opts.Policy.IgnoreNamespaces {
		d.ignored[ns] = true
	}
	inv := &api.Inventory{
		TypeMeta:    api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindInventory},
		Metadata:    api.Metadata{Name: opts.Environment},
		Environment: opts.Environment,
		CollectedAt: time.Now().UTC(),
	}
	inv.Cluster.Context = opts.Context

	if err := d.cluster(ctx, &inv.Cluster); err != nil {
		return nil, fmt.Errorf("cannot reach cluster: %w", err)
	}
	groups, err := d.apiGroups()
	if err != nil {
		inv.Errors = append(inv.Errors, "api groups: "+err.Error())
	}
	type step struct {
		name string
		fn   func(context.Context) error
	}
	steps := []step{{"workloads", d.workloads}, {"helm releases", d.helmReleases}}
	if groups["pkg.crossplane.io"] {
		steps = append(steps, step{"crossplane packages", d.crossplanePackages})
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			inv.Errors = append(inv.Errors, s.name+": "+err.Error())
		}
	}
	inv.Components = d.col.components()
	return inv, nil
}

type discoverer struct {
	kc      kubernetes.Interface
	dc      dynamic.Interface
	cat     *catalog.Catalog
	opts    Options
	col     *collector
	ignored map[string]bool
}

func (d *discoverer) cluster(ctx context.Context, info *api.ClusterInfo) error {
	sv, err := d.kc.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	info.GitVersion = sv.GitVersion
	info.Version = version.Normalize(sv.GitVersion)
	info.Distribution = distributionFromVersion(sv.GitVersion)

	nodes, err := d.kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		info.Nodes = len(nodes.Items)
		if info.Distribution == "" && len(nodes.Items) > 0 {
			info.Distribution = distributionFromNode(&nodes.Items[0])
		}
	}
	if k := d.cat.Kubernetes(); k != nil {
		d.col.add(k, api.Evidence{
			Source:  api.SourceKubernetes,
			Object:  "cluster",
			Detail:  strings.TrimSpace(sv.GitVersion + " " + info.Distribution),
			Version: info.Version,
		})
	}
	return nil
}

func distributionFromVersion(gitVersion string) string {
	switch {
	case strings.Contains(gitVersion, "-eks-"):
		return "eks"
	case strings.Contains(gitVersion, "-gke."):
		return "gke"
	case strings.Contains(gitVersion, "+k3s"):
		return "k3s"
	case strings.Contains(gitVersion, "+rke2"):
		return "rke2"
	}
	return ""
}

func distributionFromNode(n *corev1.Node) string {
	if _, ok := n.Labels["kubernetes.azure.com/cluster"]; ok {
		return "aks"
	}
	if _, ok := n.Labels["eks.amazonaws.com/nodegroup"]; ok {
		return "eks"
	}
	switch p := n.Spec.ProviderID; {
	case strings.HasPrefix(p, "azure://"):
		return "aks"
	case strings.HasPrefix(p, "aws://"):
		return "eks"
	case strings.HasPrefix(p, "gce://"):
		return "gke"
	case strings.HasPrefix(p, "kind://"):
		return "kind"
	}
	return ""
}

func (d *discoverer) apiGroups() (map[string]bool, error) {
	list, err := d.kc.Discovery().ServerGroups()
	if err != nil {
		return nil, err
	}
	groups := make(map[string]bool, len(list.Groups))
	for _, g := range list.Groups {
		groups[g.Name] = true
		for _, comp := range d.cat.MatchAPIGroup(g.Name) {
			d.col.add(comp, api.Evidence{Source: api.SourceAPIGroup, Detail: g.Name})
		}
	}
	return groups, nil
}

func (d *discoverer) workloads(ctx context.Context) error {
	apps := d.kc.AppsV1()
	opts := metav1.ListOptions{Limit: pageSize}
	for {
		l, err := apps.Deployments("").List(ctx, opts)
		if err != nil {
			return err
		}
		for i := range l.Items {
			o := &l.Items[i]
			d.podSpec(o.Namespace, "Deployment/"+o.Name, &o.Spec.Template.Spec)
		}
		if opts.Continue = l.Continue; opts.Continue == "" {
			break
		}
	}
	opts.Continue = ""
	for {
		l, err := apps.StatefulSets("").List(ctx, opts)
		if err != nil {
			return err
		}
		for i := range l.Items {
			o := &l.Items[i]
			d.podSpec(o.Namespace, "StatefulSet/"+o.Name, &o.Spec.Template.Spec)
		}
		if opts.Continue = l.Continue; opts.Continue == "" {
			break
		}
	}
	opts.Continue = ""
	for {
		l, err := apps.DaemonSets("").List(ctx, opts)
		if err != nil {
			return err
		}
		for i := range l.Items {
			o := &l.Items[i]
			d.podSpec(o.Namespace, "DaemonSet/"+o.Name, &o.Spec.Template.Spec)
		}
		if opts.Continue = l.Continue; opts.Continue == "" {
			break
		}
	}
	return nil
}

func (d *discoverer) podSpec(ns, object string, spec *corev1.PodSpec) {
	if d.ignored[ns] {
		return
	}
	containers := append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...)
	for _, c := range containers {
		repo, tag := catalog.SplitImage(c.Image)
		comp := d.cat.MatchImage(repo)
		if comp == nil {
			continue
		}
		d.col.add(comp, api.Evidence{
			Source:    api.SourceImage,
			Namespace: ns,
			Object:    object,
			Detail:    c.Image,
			Version:   tag,
		})
	}
}

func (d *discoverer) helmReleases(ctx context.Context) error {
	opts := metav1.ListOptions{Limit: pageSize, LabelSelector: "owner=helm,status=deployed"}
	for {
		l, err := d.kc.CoreV1().Secrets("").List(ctx, opts)
		if err != nil {
			return err
		}
		for i := range l.Items {
			s := &l.Items[i]
			if s.Type != "helm.sh/release.v1" || d.ignored[s.Namespace] {
				continue
			}
			rel, err := decodeHelmRelease(s.Data["release"])
			if err != nil {
				continue
			}
			d.addHelmRelease(s.Namespace, rel)
		}
		if opts.Continue = l.Continue; opts.Continue == "" {
			break
		}
	}
	return nil
}

func (d *discoverer) addHelmRelease(ns string, rel *helmRelease) {
	md := rel.Chart.Metadata
	ev := api.Evidence{
		Source:    api.SourceHelm,
		Namespace: ns,
		Object:    "HelmRelease/" + rel.Name,
		Detail:    fmt.Sprintf("chart %s-%s", md.Name, md.Version),
		Version:   md.AppVersion,
	}
	if comp := d.cat.MatchHelmChart(md.Name); comp != nil {
		d.col.add(comp, ev)
		return
	}
	if !d.opts.Policy.HideUnclassified {
		if ev.Version == "" {
			ev.Version = md.Version
		}
		d.col.addUnknown(md.Name, md.Name, "other", "", ev)
	}
}

var crossplanePackageKinds = []struct {
	resource string
	versions []string
}{
	{"providers", []string{"v1"}},
	{"functions", []string{"v1", "v1beta1"}},
	{"configurations", []string{"v1"}},
}

// classifyPackage names an uncatalogued Crossplane package and places it in the component tree.
func classifyPackage(resource, repo string) (name, display, category, partOf string) {
	name = repo[strings.LastIndex(repo, "/")+1:]
	switch resource {
	case "configurations":
		return name, name, "platform-api", ""
	case "functions":
		// Functions embedded in a configuration package are published as <configuration>_<function>.
		if cfg, fn, ok := strings.Cut(name, "_"); ok {
			return name, fn, "platform-api", cfg
		}
		return name, name, "composition", "crossplane"
	default:
		return name, name, "infrastructure", "crossplane"
	}
}

func (d *discoverer) crossplanePackages(ctx context.Context) error {
	var errs []string
	for _, k := range crossplanePackageKinds {
		var lastErr error
		for _, v := range k.versions {
			gvr := schema.GroupVersionResource{Group: "pkg.crossplane.io", Version: v, Resource: k.resource}
			l, err := d.dc.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				lastErr = err
				continue
			}
			lastErr = nil
			for _, item := range l.Items {
				pkg, _, _ := unstructured.NestedString(item.Object, "spec", "package")
				if pkg == "" {
					continue
				}
				repo, tag := catalog.SplitImage(pkg)
				ev := api.Evidence{
					Source:  api.SourceCrossplane,
					Object:  item.GetKind() + "/" + item.GetName(),
					Detail:  pkg,
					Version: tag,
				}
				if comp := d.cat.MatchCrossplanePackage(repo); comp != nil {
					d.col.add(comp, ev)
				} else {
					name, display, category, partOf := classifyPackage(k.resource, repo)
					d.col.addUnknown(name, display, category, partOf, ev)
				}
			}
			break
		}
		if lastErr != nil {
			errs = append(errs, k.resource+": "+lastErr.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// collector aggregates evidence per component.
type collector struct {
	items map[string]*api.DiscoveredComponent
}

func newCollector() *collector {
	return &collector{items: map[string]*api.DiscoveredComponent{}}
}

func (c *collector) add(comp *api.Component, ev api.Evidence) {
	dc := c.items[comp.Metadata.Name]
	if dc == nil {
		dc = &api.DiscoveredComponent{
			Name:        comp.Metadata.Name,
			DisplayName: comp.Spec.DisplayName,
			Category:    comp.Spec.Category,
			PartOf:      comp.Spec.PartOf,
			Known:       true,
		}
		c.items[comp.Metadata.Name] = dc
	}
	dc.Evidence = append(dc.Evidence, ev)
}

func (c *collector) addUnknown(name, display, category, partOf string, ev api.Evidence) {
	dc := c.items[name]
	if dc == nil {
		dc = &api.DiscoveredComponent{Name: name, DisplayName: display, Category: category, PartOf: partOf}
		c.items[name] = dc
	}
	dc.Evidence = append(dc.Evidence, ev)
}

func (c *collector) components() []api.DiscoveredComponent {
	out := make([]api.DiscoveredComponent, 0, len(c.items))
	for _, dc := range c.items {
		dc.Version = ResolveVersion(dc.Evidence)
		dc.Kind = KindOf(dc.Evidence)
		out = append(out, *dc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// KindOf describes what a component is from its most specific evidence.
func KindOf(evidence []api.Evidence) string {
	seen := map[string]string{}
	for _, e := range evidence {
		if _, ok := seen[e.Source]; !ok {
			seen[e.Source] = e.Object
		}
	}
	if obj, ok := seen[api.SourceCrossplane]; ok {
		kind, _, _ := strings.Cut(obj, "/")
		return strings.ToLower(kind)
	}
	for _, k := range [][2]string{{api.SourceKubernetes, "cluster"}, {api.SourceImage, "controller"}, {api.SourceHelm, "helm"}} {
		if _, ok := seen[k[0]]; ok {
			return k[1]
		}
	}
	return "api"
}

// sourceRank orders evidence by how trustworthy its version is.
var sourceRank = map[string]int{
	api.SourceKubernetes: 0,
	api.SourceCrossplane: 1,
	api.SourceImage:      2,
	api.SourceHelm:       3,
}

// ResolveVersion picks a component version from its evidence: the most
// trustworthy source wins, then the most frequent version, then the highest.
func ResolveVersion(evidence []api.Evidence) string {
	best := -1
	counts := map[string]int{}
	for _, ev := range evidence {
		rank, ok := sourceRank[ev.Source]
		if !ok {
			continue
		}
		v, ok := version.Parse(ev.Version)
		if !ok {
			continue
		}
		if best == -1 || rank < best {
			best = rank
			counts = map[string]int{}
		}
		if rank == best {
			counts[v.Display()]++
		}
	}
	var chosen string
	for v, n := range counts {
		if chosen == "" || n > counts[chosen] || (n == counts[chosen] && version.CompareStrings(v, chosen) > 0) {
			chosen = v
		}
	}
	return chosen
}
