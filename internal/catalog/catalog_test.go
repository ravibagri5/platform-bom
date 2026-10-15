package catalog

import "testing"

func TestBuiltinCatalogLoads(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if k := c.Kubernetes(); k == nil || k.Metadata.Name != "kubernetes" {
		t.Fatal("expected builtin kubernetes component")
	}
	for _, comp := range c.List() {
		if comp.Spec.Upstream != nil && comp.Spec.Upstream.GitHub == "" {
			t.Errorf("%s: upstream without github", comp.Metadata.Name)
		}
		if CategoryRank(comp.Spec.Category) == len(Categories) {
			t.Errorf("%s: category %q is not a platform pillar", comp.Metadata.Name, comp.Spec.Category)
		}
	}
}

func TestLegacyCategories(t *testing.T) {
	comps, err := Parse([]byte("apiVersion: pbom.dev/v1alpha1\nkind: Component\nmetadata: {name: x}\nspec: {category: compute}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if comps[0].Spec.Category != "infrastructure" {
		t.Errorf("compute -> %q", comps[0].Spec.Category)
	}
	if NormalizeCategory("composition") != "infrastructure" || NormalizeCategory("ml-platform") != "ml-platform" {
		t.Error("unexpected normalisation")
	}
}

func TestMatching(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"quay.io/argoproj/argocd":                                              "argocd",
		"registry.corp.example/mirror/argoproj/argocd":                         "argocd",
		"public.ecr.aws/karpenter/controller":                                  "karpenter",
		"registry.k8s.io/ingress-nginx/controller":                             "ingress-nginx",
		"xpkg.crossplane.io/crossplane/crossplane":                             "crossplane",
		"ghcr.io/open-telemetry/opentelemetry-operator/opentelemetry-operator": "opentelemetry-operator",
	}
	for img, want := range cases {
		got := c.MatchImage(img)
		if got == nil || got.Metadata.Name != want {
			t.Errorf("MatchImage(%q) = %v, want %s", img, got, want)
		}
	}
	if got := c.MatchCrossplanePackage("xpkg.upbound.io/upbound/provider-aws-s3"); got == nil || got.Metadata.Name != "provider-upjet-aws" {
		t.Errorf("unexpected crossplane package match: %v", got)
	}
	if c.MatchImage("docker.io/library/nginx") != nil {
		t.Error("nginx should not match")
	}
	charts := map[string]string{
		"traefik":                          "traefik",
		"ztunnel":                          "istio",
		"kiali-operator":                   "kiali",
		"thanos":                           "thanos",
		"prometheus-blackbox-exporter":     "blackbox-exporter",
		"csi-secrets-store-provider-azure": "secrets-store-provider-azure",
		"workload-identity-webhook":        "azure-workload-identity",
		"vpa":                              "vertical-pod-autoscaler",
		"strimzi-kafka-operator":           "strimzi",
		"kubernetes-dashboard":             "kubernetes-dashboard",
		"linkerd-control-plane":            "linkerd",
		"falco":                            "falco",
		"cloudnative-pg":                   "cloudnative-pg",
		"rook-ceph":                        "rook",
		"nats":                             "nats",
		"opentelemetry-collector":          "opentelemetry-collector",
		"argo-events":                      "argo-events",
		"backstage":                        "backstage",
		"argo-cd":                          "argocd",
		"spiderpool":                       "spiderpool",
	}
	for chart, want := range charts {
		if got := c.MatchHelmChart(chart); got == nil || got.Metadata.Name != want {
			t.Errorf("MatchHelmChart(%q) = %v, want %s", chart, got, want)
		}
	}
	images := map[string]string{
		"quay.io/thanos/thanos":                                 "thanos",
		"ghcr.io/dexidp/dex":                                    "dex",
		"quay.io/prometheus/blackbox-exporter":                  "blackbox-exporter",
		"mcr.microsoft.com/oss/azure/workload-identity/webhook": "azure-workload-identity",
		"ghcr.io/loft-sh/vcluster-oss":                          "vcluster",
		"docker.io/library/traefik":                             "traefik",
	}
	for img, want := range images {
		if got := c.MatchImage(img); got == nil || got.Metadata.Name != want {
			t.Errorf("MatchImage(%q) = %v, want %s", img, got, want)
		}
	}
	if got := c.MatchCrossplanePackage("xpkg.upbound.io/upbound/provider-kubernetes"); got == nil || got.Metadata.Name != "provider-kubernetes" {
		t.Errorf("upbound provider-kubernetes = %v", got)
	}
	if got, ok := c.Get("spiderpool"); !ok || got.Spec.Category != "networking" {
		t.Errorf("Get(spiderpool) = %v, %t", got, ok)
	}
	if len(c.List()) > 200 {
		t.Errorf("List() includes landscape projects: %d components", len(c.List()))
	}
}

func TestSplitImage(t *testing.T) {
	cases := [][3]string{
		{"quay.io/argoproj/argocd:v3.1.0", "quay.io/argoproj/argocd", "v3.1.0"},
		{"localhost:5000/foo/bar", "localhost:5000/foo/bar", ""},
		{"ghcr.io/kyverno/kyverno:v1.15.0@sha256:abc", "ghcr.io/kyverno/kyverno", "v1.15.0"},
	}
	for _, c := range cases {
		repo, tag := SplitImage(c[0])
		if repo != c[1] || tag != c[2] {
			t.Errorf("SplitImage(%q) = %q, %q", c[0], repo, tag)
		}
	}
}
