// Package httpapi serves the IP check endpoint and the operational endpoints.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

// Source gives the current database, or nil when none is loaded.
type Source interface {
	Current() geoipdb.Database
}

// Settings are the values the handlers need from the configuration.
type Settings struct {
	MaxBodyBytes   int64
	ProblemBaseURL string
	Version        string
}

// New returns the HTTP handler of the service.
func New(src Source, s Settings, logger *slog.Logger) http.Handler {
	m := newMetrics()
	p := problems{baseURL: s.ProblemBaseURL, metrics: m}
	mux := http.NewServeMux()
	mux.Handle("/v1/ip-checks", &checks{src: src, problems: p, metrics: m, maxBodyBytes: s.MaxBodyBytes, logger: logger})
	ops{src: src, problems: p, metrics: m, version: s.Version, logger: logger}.handle(mux)
	return recoverPanics(logger, p, mux)
}

// writeJSON sends v as JSON with the given status and content type. A HEAD
// request gets the headers only.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, contentType string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
