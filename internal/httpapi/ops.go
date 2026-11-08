package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// ops serves /livez, /readyz, /version and /metrics.
type ops struct {
	src      Source
	problems problems
	metrics  *metrics
	version  string
	logger   *slog.Logger
}

// handle registers the operational endpoints on mux.
func (o ops) handle(mux *http.ServeMux) {
	mux.HandleFunc("/livez", o.getOnly(o.livez))
	mux.HandleFunc("/readyz", o.getOnly(o.readyz))
	mux.HandleFunc("/version", o.getOnly(o.versionInfo))
	mux.HandleFunc("/metrics", o.getOnly(o.metrics.serve(o.src)))
}

// getOnly answers any method but GET and HEAD with 405 and writes a summary
// line at DEBUG level, so probes do not fill the log.
func (o ops) getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		tw := &trackingWriter{ResponseWriter: w}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			tw.Header().Set("Allow", "GET, HEAD")
			o.problems.write(tw, r, typeMethodNotAllowed, "This resource only supports GET and HEAD.", nil)
		} else {
			next(tw, r)
		}
		o.logger.LogAttrs(r.Context(), slog.LevelDebug, "request handled",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", tw.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()))
	}
}

func (o ops) livez(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, "application/json", map[string]string{"status": "ok"})
}

func (o ops) readyz(w http.ResponseWriter, r *http.Request) {
	db := o.src.Current()
	if db == nil {
		o.problems.write(w, r, typeDatabaseMissing, detailDatabaseMissing, nil)
		return
	}
	info := db.Info()
	writeJSON(w, r, http.StatusOK, "application/json", map[string]string{
		"status":            "ready",
		"database_type":     info.Type,
		"database_built_at": info.BuiltAt.UTC().Format(time.RFC3339),
	})
}

func (o ops) versionInfo(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{"version": o.version}
	if db := o.src.Current(); db != nil {
		body["database_built_at"] = db.Info().BuiltAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, r, http.StatusOK, "application/json", body)
}
