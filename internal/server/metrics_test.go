package server

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/service"
)

func TestPlatformCollector(t *testing.T) {
	svc, err := service.New("../../examples/acme/pbom.yaml", service.Options{NoUpstream: true})
	if err != nil {
		t.Fatal(err)
	}
	c := PlatformCollector{Input: svc.CachedInput}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	if n := testutil.CollectAndCount(c); n != 0 {
		t.Fatalf("collected %d series before the first discovery; scrapes must not discover", n)
	}

	if _, err := svc.Input(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if problems, err := testutil.CollectAndLint(c); err != nil || len(problems) > 0 {
		t.Fatalf("lint: %v %v", problems, err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, f := range families {
		got[f.GetName()] = len(f.GetMetric())
	}
	for _, name := range []string{"pbom_platform_info", "pbom_component_info", "pbom_component_status", "pbom_component_freshness", "pbom_environment_compliant"} {
		if got[name] == 0 {
			t.Errorf("no %s series; got %v", name, got)
		}
	}
	if got["pbom_environment_compliant"] != 3 {
		t.Errorf("compliant series = %d, want one per acme environment", got["pbom_environment_compliant"])
	}
	if got["pbom_component_upstream_info"] != 0 {
		t.Error("upstream info without upstream data")
	}

	empty := PlatformCollector{Input: func() (analysis.Input, bool) { return analysis.Input{}, false }}
	if n := testutil.CollectAndCount(empty); n != 0 {
		t.Errorf("collected %d series without input", n)
	}
}
