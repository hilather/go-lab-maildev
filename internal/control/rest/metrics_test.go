package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/observability"
)

func TestMetricsPublicPathDisabled(t *testing.T) {
	s, _ := newTestServer(t)
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/metrics", "")
	requireProblem(t, got, http.StatusNotFound, "not_found")
}

func TestMetricsPublicPathOpenMetrics(t *testing.T) {
	svc := bootTestApp(t)
	reg := observability.NewRegistry()
	reg.Inc(observability.MetricSMTPSessionsTotal, map[string]string{"result": "ok"}, 1)
	s, err := New(Config{Service: svc, RatePerSec: -1, PublicMetrics: true, Metrics: reg})
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/metrics", "")
	requireStatus(t, got, http.StatusOK)
	if !strings.Contains(got.Header().Get("Content-Type"), "openmetrics") {
		t.Fatalf("content-type=%s", got.Header().Get("Content-Type"))
	}
	body := got.Body.String()
	if !strings.Contains(body, observability.MetricSMTPSessionsTotal) || !strings.Contains(body, "# EOF") {
		t.Fatalf("body=%s", body)
	}
}

func TestHTTPRequestMetrics(t *testing.T) {
	svc := bootTestApp(t)
	reg := observability.NewRegistry()
	s, err := New(Config{Service: svc, RatePerSec: -1, Metrics: reg})
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/health/live", "")
	requireStatus(t, got, http.StatusOK)
	v, ok := reg.Get(observability.MetricHTTPRequestsTotal, map[string]string{
		"capability": "health.live",
		"code_class": "2xx",
	})
	if !ok || v < 1 {
		t.Fatalf("http requests=%v ok=%v", v, ok)
	}
}

func TestReadyOverride(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, Ready: func() bool { return false }})
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/health/ready", "")
	requireStatus(t, got, http.StatusServiceUnavailable)
	st := doReq(t, s.Handler(), http.MethodGet, "/v1/status", "")
	requireStatus(t, st, http.StatusOK)
	if decodeJSON(t, st)["ready"] != false {
		t.Fatalf("status.ready must match health hook: %s", st.Body.String())
	}
}
