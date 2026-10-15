package server

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ravibagri5/platform-bom/internal/analysis"
)

var (
	platformInfoDesc = prometheus.NewDesc("pbom_platform_info",
		"The platform PBOM serves and its newest declared release; always 1.",
		[]string{"platform", "current_release"}, nil)
	componentInfoDesc = prometheus.NewDesc("pbom_component_info",
		"A component version running in an environment; always 1.",
		[]string{"environment", "component", "version", "category"}, nil)
	componentStatusDesc = prometheus.NewDesc("pbom_component_status",
		"A component's status against the environment's target release: aligned, drift, missing, untracked, present or absent; always 1.",
		[]string{"environment", "component", "status"}, nil)
	componentFreshnessDesc = prometheus.NewDesc("pbom_component_freshness",
		"How far a running version is behind upstream: latest, patch-available, minor-behind, major-behind, unsupported or unknown; always 1.",
		[]string{"environment", "component", "freshness"}, nil)
	componentUpstreamDesc = prometheus.NewDesc("pbom_component_upstream_info",
		"The newest upstream release of a component; always 1.",
		[]string{"component", "latest"}, nil)
	environmentCompliantDesc = prometheus.NewDesc("pbom_environment_compliant",
		"1 when an environment runs its target release, 0 when not. Only environments with a declared targetRelease.",
		[]string{"environment", "target_release", "running_release"}, nil)
)

// PlatformCollector exports the platform model from the last discovery. It
// never starts discovery or upstream requests, and ignores drafts so that
// alerts only fire on what was declared.
type PlatformCollector struct {
	Input func() (analysis.Input, bool)
}

// Describe implements prometheus.Collector.
func (c PlatformCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{platformInfoDesc, componentInfoDesc, componentStatusDesc,
		componentFreshnessDesc, componentUpstreamDesc, environmentCompliantDesc} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c PlatformCollector) Collect(ch chan<- prometheus.Metric) {
	in, ok := c.Input()
	if !ok {
		return
	}
	m := analysis.BuildMatrix(in)
	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	gauge(platformInfoDesc, 1, in.Platform.Metadata.Name, m.CurrentRelease)
	for _, r := range m.Rows {
		if r.Latest != "" {
			gauge(componentUpstreamDesc, 1, r.Component, r.Latest)
		}
		for env, cell := range r.Cells {
			gauge(componentStatusDesc, 1, env, r.Component, cell.Status)
			if cell.Version == "" {
				continue
			}
			gauge(componentInfoDesc, 1, env, r.Component, cell.Version, r.Category)
			gauge(componentFreshnessDesc, 1, env, r.Component, string(cell.Freshness))
		}
	}
	for _, e := range m.Environments {
		if e.TargetRelease == "" {
			continue
		}
		v := 0.0
		if e.Compliant {
			v = 1
		}
		gauge(environmentCompliantDesc, v, e.Name, e.TargetRelease, e.MatchedRelease)
	}
}
