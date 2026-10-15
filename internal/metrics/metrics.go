// Package metrics holds the Prometheus metrics about PBOM itself: discovery,
// upstream requests, configuration reloads and HTTP traffic. Metrics about the
// platform are computed from the analysis by the server package.
package metrics

import (
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry holds every PBOM metric, plus Go runtime and process metrics.
var Registry = prometheus.NewRegistry()

var (
	buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pbom_build_info",
		Help: "PBOM version; always 1.",
	}, []string{"version", "goversion"})

	discoveryDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pbom_discovery_duration_seconds",
		Help: "Duration of the last discovery of an environment.",
	}, []string{"environment"})
	discoveryLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pbom_discovery_last_success_timestamp_seconds",
		Help: "Unix time of the last successful discovery of an environment.",
	}, []string{"environment"})
	discoveryErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pbom_discovery_errors_total",
		Help: "Discoveries of an environment that failed.",
	}, []string{"environment"})

	upstreamRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pbom_upstream_requests_total",
		Help: "Upstream release lookups by source (api, feed) and result (ok, error, rate_limited).",
	}, []string{"source", "result"})

	configReloads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pbom_config_reloads_total",
		Help: "Configuration loads by result (success, failure).",
	}, []string{"result"})
	configLastReloadSuccessful = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pbom_config_last_reload_successful",
		Help: "Whether the last configuration load succeeded; on failure the previous configuration keeps serving.",
	})

	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pbom_http_requests_total",
		Help: "HTTP requests to the UI and API by method and status code.",
	}, []string{"method", "code"})
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pbom_http_request_duration_seconds",
		Help:    "Duration of HTTP requests to the UI and API.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"method"})
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo, discoveryDuration, discoveryLastSuccess, discoveryErrors,
		upstreamRequests, configReloads, configLastReloadSuccessful, httpRequests, httpDuration,
	)
}

// SetBuildInfo records the running version.
func SetBuildInfo(version string) {
	buildInfo.WithLabelValues(version, runtime.Version()).Set(1)
}

// ObserveDiscovery records one discovery of env.
func ObserveDiscovery(env string, d time.Duration, err error) {
	discoveryDuration.WithLabelValues(env).Set(d.Seconds())
	if err != nil {
		discoveryErrors.WithLabelValues(env).Inc()
		return
	}
	discoveryErrors.WithLabelValues(env).Add(0)
	discoveryLastSuccess.WithLabelValues(env).SetToCurrentTime()
}

// Upstream sources and results.
const (
	SourceAPI         = "api"
	SourceFeed        = "feed"
	ResultOK          = "ok"
	ResultError       = "error"
	ResultRateLimited = "rate_limited"
)

// UpstreamRequest records one upstream release lookup.
func UpstreamRequest(source, result string) {
	upstreamRequests.WithLabelValues(source, result).Inc()
}

// ConfigReload records a configuration load.
func ConfigReload(ok bool) {
	if ok {
		configReloads.WithLabelValues("success").Inc()
		configLastReloadSuccessful.Set(1)
		return
	}
	configReloads.WithLabelValues("failure").Inc()
	configLastReloadSuccessful.Set(0)
}

// InstrumentHTTP counts and times the requests h serves.
func InstrumentHTTP(h http.Handler) http.Handler {
	return promhttp.InstrumentHandlerCounter(httpRequests, promhttp.InstrumentHandlerDuration(httpDuration, h))
}

// Handler serves the registry in the Prometheus exposition format.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{Registry: Registry})
}
