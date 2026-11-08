package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func scrape(t *testing.T, h *harness) string {
	t.Helper()
	rec := h.do(http.MethodGet, "/metrics", "", "")
	if rec.Code != 200 {
		t.Fatalf("/metrics status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain; version=0.0.4") {
		t.Errorf("Content-Type = %q, want text/plain; version=0.0.4", got)
	}
	return rec.Body.String()
}

func wantLine(t *testing.T, text, line string) {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		if l == line {
			return
		}
	}
	t.Errorf("metrics have no line %q:\n%s", line, text)
}

func TestMetricsCountChecksByReason(t *testing.T) {
	h := newHarness(gbDB)
	h.post(validBody)
	h.post(`{"ip_address":"81.2.69.160","allowed_countries":["DE"]}`)
	h.post(`{"ip_address":"81.2.69.160","allowed_countries":["DE"]}`)

	text := scrape(t, h)
	wantLine(t, text, `ip_checks_total{reason="country_allowed"} 1`)
	wantLine(t, text, `ip_checks_total{reason="country_not_allowed"} 2`)
	wantLine(t, text, `ip_checks_total{reason="country_unknown"} 0`)
	wantLine(t, text, `ip_checks_total{reason="country_unsupported"} 0`)
	wantLine(t, text, `# TYPE ip_checks_total counter`)
}

func TestMetricsCountProblemsByType(t *testing.T) {
	h := newHarness(gbDB)
	h.post(`{}`)
	h.post(`not json`)

	text := scrape(t, h)
	wantLine(t, text, `http_problems_total{type="invalid-request"} 1`)
	wantLine(t, text, `http_problems_total{type="malformed-request"} 1`)
	wantLine(t, text, `http_problems_total{type="internal-error"} 0`)
}

func TestMetricsDatabaseBuildTimestamp(t *testing.T) {
	wantLine(t, scrape(t, newHarness(gbDB)), `database_build_timestamp_seconds 1767323045`)

	if text := scrape(t, newHarness(nil)); strings.Contains(text, "database_build_timestamp_seconds ") {
		t.Errorf("metrics have a build timestamp without a database:\n%s", text)
	}
}

func TestMetricsOnlyAllowGetAndHead(t *testing.T) {
	rec := newHarness(gbDB).do(http.MethodPost, "/metrics", "", "")
	wantProblem(t, rec, 405, "method-not-allowed", "Method not allowed", "This resource only supports GET and HEAD.")
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
	}
}

func TestMetricsHeadHasNoBody(t *testing.T) {
	rec := newHarness(gbDB).do(http.MethodHead, "/metrics", "", "")
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("got %d with %d body bytes, want 200 with none", rec.Code, rec.Body.Len())
	}
}

// TestMetricsCountMethodNotAllowedOnOperationalEndpoints covers G69: a 405 on an
// operational endpoint counts in http_problems_total.
func TestMetricsCountMethodNotAllowedOnOperationalEndpoints(t *testing.T) {
	h := newHarness(gbDB)
	h.do(http.MethodPost, "/livez", "", "")
	h.do(http.MethodPut, "/readyz", "", "")

	wantLine(t, scrape(t, h), `http_problems_total{type="method-not-allowed"} 2`)
}
