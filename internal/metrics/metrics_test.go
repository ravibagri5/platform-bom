package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecorders(t *testing.T) {
	ObserveDiscovery("prod", 2*time.Second, nil)
	ObserveDiscovery("prod", time.Second, errors.New("unreachable"))
	if got := testutil.ToFloat64(discoveryErrors.WithLabelValues("prod")); got != 1 {
		t.Errorf("discovery errors = %v", got)
	}
	if got := testutil.ToFloat64(discoveryDuration.WithLabelValues("prod")); got != 1 {
		t.Errorf("last duration = %v", got)
	}
	if testutil.ToFloat64(discoveryLastSuccess.WithLabelValues("prod")) == 0 {
		t.Error("last success not recorded")
	}

	UpstreamRequest(SourceAPI, ResultRateLimited)
	if got := testutil.ToFloat64(upstreamRequests.WithLabelValues(SourceAPI, ResultRateLimited)); got != 1 {
		t.Errorf("rate limited = %v", got)
	}

	ConfigReload(false)
	if testutil.ToFloat64(configLastReloadSuccessful) != 0 {
		t.Error("failed reload not recorded")
	}
	ConfigReload(true)
	if testutil.ToFloat64(configLastReloadSuccessful) != 1 {
		t.Error("successful reload not recorded")
	}

	h := InstrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if got := testutil.ToFloat64(httpRequests.WithLabelValues("get", "418")); got != 1 {
		t.Errorf("http requests = %v", got)
	}

	SetBuildInfo("1.2.3")
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{`pbom_build_info{goversion="`, `version="1.2.3"`, "pbom_discovery_errors_total", "go_goroutines"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("/metrics lacks %s", want)
		}
	}
}
