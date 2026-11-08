package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

const (
	baseURL   = "https://geo-service.example/problems/"
	validBody = `{"ip_address":"81.2.69.160","allowed_countries":["GB"]}`
)

var builtAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

type fakeDB struct {
	record geoipdb.Record
	err    error
}

func (f fakeDB) Info() geoipdb.Info {
	return geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt}
}

func (f fakeDB) Lookup(netip.Addr) (geoipdb.Record, error) { return f.record, f.err }

type fakeSource struct{ db geoipdb.Database }

func (s fakeSource) Current() geoipdb.Database { return s.db }

var gbDB = fakeDB{record: geoipdb.Record{Found: true, ISOCode: "GB", Name: "United Kingdom"}}

type harness struct {
	handler http.Handler
	logs    *bytes.Buffer
	logger  *slog.Logger
}

// panicDB panics on lookup, to test the recovery middleware.
type panicDB struct{ fakeDB }

func (panicDB) Lookup(netip.Addr) (geoipdb.Record, error) { panic("boom secret") }

// newHarness builds the handler over db (nil means no database) with a JSON
// logger at DEBUG level.
func newHarness(db geoipdb.Database) *harness {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	settings := Settings{MaxBodyBytes: 16384, ProblemBaseURL: baseURL, Version: "1.2.3"}
	return &harness{handler: New(fakeSource{db: db}, settings, logger), logs: logs, logger: logger}
}

func (h *harness) do(method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) post(body string) *httptest.ResponseRecorder {
	return h.do(http.MethodPost, "/v1/ip-checks", "application/json", body)
}

// records returns the decoded JSON log lines.
func (h *harness) records(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(h.logs.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// summary returns the one summary line of the request.
func (h *harness) summary(t *testing.T) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, rec := range h.records(t) {
		if rec["msg"] == "request handled" {
			found = append(found, rec)
		}
	}
	if len(found) != 1 {
		t.Fatalf("summary lines = %d, want 1; logs:\n%s", len(found), h.logs)
	}
	return found[0]
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", rec.Body, err)
	}
	return m
}

func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, name, title, detail string) map[string]any {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	body := decodeMap(t, rec)
	if body["type"] != baseURL+name || body["title"] != title || body["detail"] != detail || body["status"] != float64(status) {
		t.Errorf("problem = %v, want type %s%s title %q detail %q status %d", body, baseURL, name, title, detail, status)
	}
	return body
}

func TestAllowedResponse(t *testing.T) {
	h := newHarness(gbDB)
	rec := h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"session_id":"abc"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	want := map[string]any{
		"allowed":    true,
		"reason":     "country_allowed",
		"detail":     "The IP address is located in GB (United Kingdom), which is in allowed_countries.",
		"country":    map[string]any{"iso_code": "GB", "name": "United Kingdom"},
		"session_id": "abc",
	}
	if got := decodeMap(t, rec); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestDecisionResponses(t *testing.T) {
	tests := []struct {
		name        string
		db          fakeDB
		body        string
		wantReason  string
		wantCountry any
		wantLevel   string
		wantCat     string
	}{
		{"not allowed", gbDB, `{"ip_address":"81.2.69.160","allowed_countries":["DE"]}`, "country_not_allowed",
			map[string]any{"iso_code": "GB", "name": "United Kingdom"}, "INFO", ""},
		{"unsupported XK", fakeDB{record: geoipdb.Record{Found: true, ISOCode: "XK", Name: "Kosovo"}}, validBody, "country_unsupported",
			map[string]any{"iso_code": "XK", "name": "Kosovo"}, "WARN", ""},
		{"public unmapped", fakeDB{}, `{"ip_address":"8.8.8.8","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "public_unmapped"},
		{"private", gbDB, `{"ip_address":"10.0.0.1","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "private"},
		{"loopback", gbDB, `{"ip_address":"127.0.0.1","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "loopback"},
		{"link local", gbDB, `{"ip_address":"fe80::1%eth0","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "link_local"},
		{"unspecified", gbDB, `{"ip_address":"0.0.0.0","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "unspecified"},
		{"reserved", gbDB, `{"ip_address":"224.0.0.1","allowed_countries":["GB"]}`, "country_unknown", nil, "WARN", "reserved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(tt.db)
			rec := h.post(tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
			}
			body := decodeMap(t, rec)
			if body["reason"] != tt.wantReason || body["allowed"] != false {
				t.Errorf("reason/allowed = %v/%v, want %s/false", body["reason"], body["allowed"], tt.wantReason)
			}
			if !reflect.DeepEqual(body["country"], tt.wantCountry) {
				t.Errorf("country = %v, want %v", body["country"], tt.wantCountry)
			}
			if _, ok := body["country"]; !ok {
				t.Error("country key is missing, want null")
			}
			sum := h.summary(t)
			if sum["level"] != tt.wantLevel || sum["reason"] != tt.wantReason {
				t.Errorf("summary level/reason = %v/%v, want %s/%s", sum["level"], sum["reason"], tt.wantLevel, tt.wantReason)
			}
			if got, _ := sum["address_category"].(string); got != tt.wantCat {
				t.Errorf("address_category = %q, want %q", got, tt.wantCat)
			}
		})
	}
}

func TestSummaryLine(t *testing.T) {
	h := newHarness(gbDB)
	h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB","IT","de"],"session_id":"abc","requested_at":"2026-11-08T10:00:00.5+01:00"}`)
	sum := h.summary(t)
	want := map[string]any{
		"level": "INFO", "method": "POST", "path": "/v1/ip-checks", "status": float64(200), "reason": "country_allowed",
		"allowed": true, "country_iso": "GB", "allowed_countries_count": float64(3),
		"requested_at": "2026-11-08T10:00:00.5+01:00", "session_id": "abc",
	}
	for k, v := range want {
		if sum[k] != v {
			t.Errorf("summary[%s] = %v, want %v", k, sum[k], v)
		}
	}
	if _, ok := sum["duration_ms"]; !ok {
		t.Error("summary has no duration_ms")
	}
}

func TestRequestedAtIsNotReturned(t *testing.T) {
	rec := newHarness(gbDB).post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"requested_at":"2026-11-08T10:00:00.5+01:00"}`)
	if strings.Contains(rec.Body.String(), "2026-11-08") || strings.Contains(rec.Body.String(), "requested_at") {
		t.Errorf("body %s contains requested_at", rec.Body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := newHarness(gbDB).do(http.MethodGet, "/v1/ip-checks", "", "")
	wantProblem(t, rec, 405, "method-not-allowed", "Method not allowed", "This resource only supports POST.")
	if got := rec.Header().Get("Allow"); got != "POST" {
		t.Errorf("Allow = %q, want POST", got)
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        int
	}{
		{"", 415},
		{"text/plain", 415},
		{"application/xml", 415},
		{"application/json; charset=utf-8", 200},
		{"Application/JSON", 200},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			rec := newHarness(gbDB).do(http.MethodPost, "/v1/ip-checks", tt.contentType, validBody)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == 415 {
				wantProblem(t, rec, 415, "unsupported-media-type", "Unsupported media type", "The request Content-Type must be application/json.")
			}
		})
	}
}

// TestHeadIsNotAllowedOnCheck covers G64: HEAD is a method that is not POST.
func TestHeadIsNotAllowedOnCheck(t *testing.T) {
	rec := newHarness(gbDB).do(http.MethodHead, "/v1/ip-checks", "application/json", validBody)
	if rec.Code != 405 {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "POST" {
		t.Errorf("Allow = %q, want POST", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
}

// TestBodyReadError covers G63: a body that cannot be read, for a reason other
// than the size limit, is answered as malformed-request.
func TestBodyReadError(t *testing.T) {
	h := newHarness(gbDB)
	req := httptest.NewRequest(http.MethodPost, "/v1/ip-checks", iotest.ErrReader(errors.New("connection reset")))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	wantProblem(t, rec, 400, "malformed-request", "Malformed request", "The request body is not valid JSON.")
}

func TestBodySizeLimit(t *testing.T) {
	padded := func(n int) string { return validBody + strings.Repeat(" ", n-len(validBody)) }
	const detail = "The request body exceeds the maximum allowed size of 16384 bytes."

	t.Run("exactly the cap", func(t *testing.T) {
		if rec := newHarness(gbDB).post(padded(16384)); rec.Code != 200 {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
	t.Run("one byte over, known length", func(t *testing.T) {
		wantProblem(t, newHarness(gbDB).post(padded(16385)), 413, "request-too-large", "Request too large", detail)
	})
	t.Run("one byte over, unknown length", func(t *testing.T) {
		h := newHarness(gbDB)
		req := httptest.NewRequest(http.MethodPost, "/v1/ip-checks", strings.NewReader(padded(16385)))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = -1
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		wantProblem(t, rec, 413, "request-too-large", "Request too large", detail)
	})
	t.Run("unknown length, exactly the cap", func(t *testing.T) {
		h := newHarness(gbDB)
		req := httptest.NewRequest(http.MethodPost, "/v1/ip-checks", strings.NewReader(padded(16384)))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = -1
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

func TestInvalidBodies(t *testing.T) {
	const invalidDetail = "The request body is missing required fields or has invalid values."
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantParams []any
	}{
		{"not json", `not json`, 400, nil},
		{"empty body", ``, 400, nil},
		{"array", `[]`, 422, []any{map[string]any{"pointer": "", "reason": "The request body must be a JSON object."}}},
		{"empty object", `{}`, 422, []any{
			map[string]any{"pointer": "/ip_address", "reason": "ip_address is required."},
			map[string]any{"pointer": "/allowed_countries", "reason": "allowed_countries is required."},
		}},
		{"empty list", `{"ip_address":"81.2.69.160","allowed_countries":[]}`, 422, []any{
			map[string]any{"pointer": "/allowed_countries", "reason": "allowed_countries must contain at least one country code."},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newHarness(gbDB).post(tt.body)
			if tt.wantStatus == 400 {
				wantProblem(t, rec, 400, "malformed-request", "Malformed request", "The request body is not valid JSON.")
				return
			}
			body := wantProblem(t, rec, 422, "invalid-request", "Invalid request", invalidDetail)
			if !reflect.DeepEqual(body["invalid_params"], tt.wantParams) {
				t.Errorf("invalid_params = %v, want %v", body["invalid_params"], tt.wantParams)
			}
		})
	}
}

func TestNoDatabase(t *testing.T) {
	t.Run("valid request gets 503", func(t *testing.T) {
		h := newHarness(nil)
		wantProblem(t, h.post(validBody), 503, "database-unavailable", "Service unavailable", "The geolocation database is not loaded yet; retry later.")
		if h.summary(t)["level"] != "ERROR" {
			t.Errorf("summary level = %v, want ERROR", h.summary(t)["level"])
		}
	})
	t.Run("invalid request gets 422 first", func(t *testing.T) {
		wantProblem(t, newHarness(nil).post(`{}`), 422, "invalid-request", "Invalid request",
			"The request body is missing required fields or has invalid values.")
	})
}

func TestLookupError(t *testing.T) {
	h := newHarness(fakeDB{err: errors.New("boom secret")})
	rec := h.post(validBody)
	wantProblem(t, rec, 500, "internal-error", "Internal error", "An unexpected error occurred.")
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body %s leaks the error", rec.Body)
	}
	sum := h.summary(t)
	if sum["level"] != "ERROR" || sum["status"] != float64(500) {
		t.Errorf("summary = %v, want ERROR 500", sum)
	}
}

func TestWarningsAndSession(t *testing.T) {
	long := strings.Repeat("é", 129)
	t.Run("valid session is echoed and on every line", func(t *testing.T) {
		h := newHarness(gbDB)
		rec := h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"session_id":"abc","foo":1}`)
		if got := decodeMap(t, rec)["session_id"]; got != "abc" {
			t.Errorf("session_id = %v, want abc", got)
		}
		recs := h.records(t)
		if len(recs) < 2 {
			t.Fatalf("log lines = %d, want at least 2", len(recs))
		}
		for _, r := range recs {
			if r["session_id"] != "abc" {
				t.Errorf("log line %v has no session_id=abc", r)
			}
		}
	})
	t.Run("too long session is dropped with a warning", func(t *testing.T) {
		h := newHarness(gbDB)
		body := decodeMap(t, h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"session_id":"`+long+`"}`))
		if _, ok := body["session_id"]; ok {
			t.Error("session_id was echoed")
		}
		warnings, _ := body["warnings"].([]any)
		if len(warnings) != 1 || warnings[0].(map[string]any)["code"] != "invalid_session_id" {
			t.Errorf("warnings = %v, want one invalid_session_id", body["warnings"])
		}
		var logged bool
		for _, r := range h.records(t) {
			if r["level"] == "WARN" && r["session_id_length"] == float64(129) {
				logged = true
			}
			if _, ok := r["session_id"]; ok {
				t.Errorf("log line %v carries an invalid session_id", r)
			}
		}
		if !logged {
			t.Errorf("no WARN line with session_id_length=129; logs:\n%s", h.logs)
		}
	})
	t.Run("non-string requested_at logs its type", func(t *testing.T) {
		h := newHarness(gbDB)
		h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"requested_at":5}`)
		for _, r := range h.records(t) {
			if r["level"] == "WARN" && r["requested_at_type"] == "number" {
				return
			}
		}
		t.Errorf("no WARN line with requested_at_type=number; logs:\n%s", h.logs)
	})
	t.Run("no warnings means no warnings key", func(t *testing.T) {
		if _, ok := decodeMap(t, newHarness(gbDB).post(validBody))["warnings"]; ok {
			t.Error("warnings key present")
		}
	})
	t.Run("duplicates warn", func(t *testing.T) {
		body := decodeMap(t, newHarness(gbDB).post(`{"ip_address":"81.2.69.160","allowed_countries":["GB","gb"]}`))
		if warnings, _ := body["warnings"].([]any); len(warnings) != 1 {
			t.Errorf("warnings = %v, want one", body["warnings"])
		}
	})
}

func TestUnknownFieldsLoggedAtDebug(t *testing.T) {
	h := newHarness(gbDB)
	h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"foo":"secret-value","bar":1}`)
	for _, r := range h.records(t) {
		if r["level"] == "DEBUG" && r["unknown_fields_count"] == float64(2) {
			if _, ok := r["unknown_fields"]; ok {
				t.Error("unknown field names were logged")
			}
			if strings.Contains(h.logs.String(), "secret-value") {
				t.Error("an unknown field value was logged")
			}
			return
		}
	}
	t.Errorf("no DEBUG line with unknown_fields_count; logs:\n%s", h.logs)
}

// TestUnknownFieldNamesAreNotLogged: names are caller-controlled, so an
// address used as a field name must not reach the logs (G75).
func TestUnknownFieldNamesAreNotLogged(t *testing.T) {
	h := newHarness(gbDB)
	h.post(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"81.2.69.160":"ignored","foo":1}`)
	if strings.Contains(h.logs.String(), "81.2.69.160") || strings.Contains(h.logs.String(), "foo") {
		t.Errorf("an unknown field name was logged:\n%s", h.logs)
	}
}

func TestUnknownPath(t *testing.T) {
	rec := newHarness(gbDB).do(http.MethodGet, "/nope", "", "")
	if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != "404 page not found" {
		t.Errorf("got %d %q, want 404 %q", rec.Code, rec.Body, "404 page not found")
	}
}

func TestIPNeverInLogsOrResponses(t *testing.T) {
	const ip = "81.2.69.160"
	bodies := []string{
		validBody,
		`{"ip_address":"81.2.69.160","allowed_countries":["USA"]}`,
		`{"ip_address":"81.2.69.160","allowed_countries":["DE"],"session_id":"` + strings.Repeat("x", 129) + `"}`,
		`{"ip_address":"81.2.69.160"}`,
	}
	for _, db := range []geoipdb.Database{gbDB, nil, fakeDB{err: errors.New("lookup failed")}} {
		h := newHarness(db)
		for _, body := range bodies {
			rec := h.post(body)
			if strings.Contains(rec.Body.String(), ip) {
				t.Errorf("response %s contains the IP", rec.Body)
			}
		}
		if strings.Contains(h.logs.String(), ip) {
			t.Errorf("logs contain the IP:\n%s", h.logs)
		}
	}
}

// TestTrailingDataAfterJSON covers G31: data after the JSON value is malformed.
func TestTrailingDataAfterJSON(t *testing.T) {
	for _, body := range []string{validBody + ` {}`, validBody + `x`, `{}{}`} {
		wantProblem(t, newHarness(gbDB).post(body), 400, "malformed-request", "Malformed request", "The request body is not valid JSON.")
	}
}
