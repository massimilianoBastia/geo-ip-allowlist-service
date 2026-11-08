package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/decision"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

const (
	maxLoggedUnknownFields = 20
	detailDatabaseMissing  = "The geolocation database is not loaded yet; retry later."
)

// checks handles POST /v1/ip-checks.
type checks struct {
	src          Source
	problems     problems
	metrics      *metrics
	maxBodyBytes int64
	logger       *slog.Logger
}

// checkResponse is the JSON body of a completed check.
type checkResponse struct {
	Allowed   bool              `json:"allowed"`
	Reason    decision.Reason   `json:"reason"`
	Detail    string            `json:"detail"`
	Country   *decision.Country `json:"country"`
	SessionID string            `json:"session_id,omitempty"`
	Warnings  []request.Warning `json:"warnings,omitempty"`
}

// outcome is what the summary log line needs to know about a request.
type outcome struct {
	status int
	req    *request.Request
	result *decision.Result
	err    error
}

func (c *checks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	out := c.serve(w, r)
	c.log(r, start, out)
}

// serve runs the pipeline of the design notes (H4) and writes the response.
func (c *checks) serve(w http.ResponseWriter, r *http.Request) outcome {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		return c.reject(w, r, typeMethodNotAllowed, "This resource only supports POST.")
	}
	if !isJSON(r.Header.Get("Content-Type")) {
		return c.reject(w, r, typeUnsupportedMedia, "The request Content-Type must be application/json.")
	}
	body, err := c.readBody(w, r)
	switch {
	case errors.Is(err, errTooLarge):
		return c.reject(w, r, typeTooLarge, fmt.Sprintf("The request body exceeds the maximum allowed size of %d bytes.", c.maxBodyBytes))
	case err != nil:
		return c.reject(w, r, typeMalformed, "The request body is not valid JSON.")
	}

	req, err := request.Parse(body)
	var invalid *request.InvalidError
	switch {
	case errors.Is(err, request.ErrMalformed):
		return c.reject(w, r, typeMalformed, "The request body is not valid JSON.")
	case errors.As(err, &invalid):
		return c.rejectInvalid(w, r, invalid.Params)
	case err != nil:
		return c.fail(w, r, nil, err)
	}
	setSessionID(r.Context(), req.SessionID)

	db := c.src.Current()
	if db == nil {
		out := c.reject(w, r, typeDatabaseMissing, detailDatabaseMissing)
		out.req = &req
		return out
	}
	result, err := decision.Decide(db, req.Addr, req.Countries)
	if err != nil {
		return c.fail(w, r, &req, err)
	}
	c.metrics.countCheck(result.Reason)
	writeJSON(w, r, http.StatusOK, "application/json", checkResponse{
		Allowed:   result.Allowed,
		Reason:    result.Reason,
		Detail:    result.Detail,
		Country:   result.Country,
		SessionID: req.SessionID,
		Warnings:  req.Warnings,
	})
	return outcome{status: http.StatusOK, req: &req, result: &result}
}

var errTooLarge = errors.New("request body too large")

// readBody reads the body up to the cap. The declared length is checked first.
func (c *checks) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength > c.maxBodyBytes {
		return nil, errTooLarge
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, c.maxBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, errTooLarge
	}
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	return body, nil
}

func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

func (c *checks) reject(w http.ResponseWriter, r *http.Request, t problemType, detail string) outcome {
	c.problems.write(w, r, t, detail, nil)
	return outcome{status: t.status}
}

func (c *checks) rejectInvalid(w http.ResponseWriter, r *http.Request, params []request.InvalidParam) outcome {
	c.problems.write(w, r, typeInvalid, "The request body is missing required fields or has invalid values.", params)
	return outcome{status: typeInvalid.status}
}

func (c *checks) fail(w http.ResponseWriter, r *http.Request, req *request.Request, err error) outcome {
	out := c.reject(w, r, typeInternal, "An unexpected error occurred.")
	out.req, out.err = req, err
	return out
}

// log writes the summary line and the lines about ignored and unknown fields.
// It never writes the IP address.
func (c *checks) log(r *http.Request, start time.Time, out outcome) {
	logger := c.logger
	if out.req != nil && out.req.SessionID != "" {
		logger = logger.With("session_id", out.req.SessionID)
	}
	ctx := r.Context()
	if out.req != nil {
		logIgnored(logger, out.req)
	}

	level := slog.LevelInfo
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", out.status),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	}
	if out.status >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	if out.err != nil {
		attrs = append(attrs, slog.Any("error", out.err))
	}
	if out.req != nil {
		attrs = append(attrs, slog.Int("allowed_countries_count", len(out.req.Countries)))
		if out.req.RequestedAt != "" {
			attrs = append(attrs, slog.String("requested_at", out.req.RequestedAt))
		}
	}
	if res := out.result; res != nil {
		attrs = append(attrs, slog.String("reason", string(res.Reason)), slog.Bool("allowed", res.Allowed))
		if res.Country != nil {
			attrs = append(attrs, slog.String("country_iso", res.Country.ISOCode))
		}
		if res.Category != "" {
			attrs = append(attrs, slog.String("address_category", string(res.Category)))
		}
		if res.Reason == decision.ReasonUnknown || res.Reason == decision.ReasonUnsupported {
			level = slog.LevelWarn
		}
	}
	logger.LogAttrs(ctx, level, "request handled", attrs...)
}

// logIgnored logs the optional fields that were ignored (WARN) and the names
// of unknown fields (DEBUG), never their values.
func logIgnored(logger *slog.Logger, req *request.Request) {
	for _, f := range req.Ignored {
		if f.JSONType != "" {
			logger.Warn("optional field ignored", "field", f.Name, f.Name+"_type", f.JSONType)
		} else {
			logger.Warn("optional field ignored", "field", f.Name, f.Name+"_length", f.Length)
		}
	}
	if n := len(req.UnknownFields); n > 0 {
		names := req.UnknownFields[:min(n, maxLoggedUnknownFields)]
		logger.Debug("unknown fields ignored", "unknown_fields", names, "unknown_fields_count", n)
	}
}
