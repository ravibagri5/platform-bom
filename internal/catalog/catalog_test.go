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
