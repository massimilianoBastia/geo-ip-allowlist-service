package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/decision"
)

// metrics counts completed checks by reason and problems by type, and renders
// them in the Prometheus text format. The maps are filled once and never
// change afterwards, so only the counters need to be atomic.
type metrics struct {
	checks   map[decision.Reason]*atomic.Int64
	problems map[string]*atomic.Int64
}

var allReasons = []decision.Reason{
	decision.ReasonAllowed, decision.ReasonNotAllowed, decision.ReasonUnknown, decision.ReasonUnsupported,
}

func newMetrics() *metrics {
	m := &metrics{
		checks:   make(map[decision.Reason]*atomic.Int64),
		problems: make(map[string]*atomic.Int64),
	}
	for _, r := range allReasons {
		m.checks[r] = new(atomic.Int64)
	}
	for _, t := range allProblemTypes {
		m.problems[t.name] = new(atomic.Int64)
	}
	return m
}

// countCheck and countProblem do nothing on a nil *metrics.
func (m *metrics) countCheck(reason decision.Reason) {
	if m != nil {
		m.checks[reason].Add(1)
	}
}

func (m *metrics) countProblem(t problemType) {
	if m != nil {
		m.problems[t.name].Add(1)
	}
}

// serve renders the metrics. The build time series is left out while no
// database is loaded.
func (m *metrics) serve(src Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		b.WriteString("# HELP ip_checks_total Completed IP checks by reason.\n# TYPE ip_checks_total counter\n")
		for _, reason := range allReasons {
			fmt.Fprintf(&b, "ip_checks_total{reason=%q} %d\n", reason, m.checks[reason].Load())
		}
		b.WriteString("# HELP http_problems_total Problem responses by type.\n# TYPE http_problems_total counter\n")
		for _, t := range allProblemTypes {
			fmt.Fprintf(&b, "http_problems_total{type=%q} %d\n", t.name, m.problems[t.name].Load())
		}
		if db := src.Current(); db != nil {
			b.WriteString("# HELP database_build_timestamp_seconds Build time of the loaded database, Unix seconds.\n# TYPE database_build_timestamp_seconds gauge\n")
			fmt.Fprintf(&b, "database_build_timestamp_seconds %d\n", db.Info().BuiltAt.Unix())
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(b.String()))
		}
	}
}
