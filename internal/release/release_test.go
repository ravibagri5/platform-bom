package release

import (
	"testing"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
)

func TestCompare(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	a := &api.PlatformRelease{Metadata: api.Metadata{Name: "v1"}, Spec: api.ReleaseSpec{
		Components: map[string]string{"kubernetes": "1.33", "argocd": "3.0.0", "istio": "1.26"},
		Offerings:  []string{"gitops"},
	}}
	b := &api.PlatformRelease{Metadata: api.Metadata{Name: "v2"}, Spec: api.ReleaseSpec{
		Components: map[string]string{"kubernetes": "1.34", "argocd": "3.0.0", "karpenter": "1.6.0"},
		Offerings:  []string{"gitops", "autoscaling"},
	}}
	d := Compare(a, b, cat)
	got := map[string]string{}
	for _, c := range d.Components {
		got[c.Component] = c.Change
	}
	want := map[string]string{"kubernetes": ChangeUpgraded, "argocd": ChangeUnchanged, "istio": ChangeRemoved, "karpenter": ChangeAdded}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %s, want %s", k, got[k], v)
		}
	}
	if len(d.OfferingsAdded) != 1 || d.OfferingsAdded[0] != "autoscaling" || len(d.OfferingsRemoved) != 0 {
		t.Errorf("unexpected offerings diff: %+v / %+v", d.OfferingsAdded, d.OfferingsRemoved)
	}
}

func TestValidName(t *testing.T) {
	for _, n := range []string{"v1.0.0", "2026.09", "rc-1"} {
		if !ValidName(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	for _, n := range []string{"", "../x", "a/b", ".hidden"} {
		if ValidName(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}
