// Package scenarios holds the acceptance tests. They assemble the real
// database reader, reloader and HTTP handler around the synthetic test
// database; nothing opens a socket.
package scenarios

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/httpapi"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/reload"
)

const (
	testDBPath = "../../testdata/GeoIP2-Country-Test.mmdb"
	baseURL    = "https://geo-service.example/problems/"
)

// fixtureIPs are the addresses used below; none may ever appear in a log.
var fixtureIPs = []string{
	"81.2.69.160", "2.125.160.216", "2001:218::1", "2a02:cf40::1", "214.1.1.1",
	"2a02:d500::1", "5.83.124.1", "8.8.8.8", "10.0.0.1", "127.0.0.1", "169.254.1.1",
	"0.0.0.0", "224.0.0.1", "fe80::1",
}

type scenario struct {
	handler http.Handler
	logs    *bytes.Buffer
}

// newScenario loads the test database through the reloader, with the sanity
// pair 81.2.69.160=GB.
func newScenario(t *testing.T) *scenario {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sanity := []geoipdb.SanityCheck{{Addr: netip.MustParseAddr("81.2.69.160"), ISOCode: "GB"}}
	reloader := reload.New(testDBPath, sanity, reload.FileStat{}, reload.FileOpener{}, logger)
	reloader.Check()
	if reloader.Current() == nil {
		t.Fatalf("test database was not loaded; logs:\n%s", logs)
	}
	handler := httpapi.New(reloader, httpapi.Settings{MaxBodyBytes: 16384, ProblemBaseURL: baseURL, Version: "test"}, logger)
	return &scenario{handler: handler, logs: logs}
}

func (s *scenario) do(method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func (s *scenario) check(ip, countries string) *httptest.ResponseRecorder {
	return s.post(`{"ip_address":"` + ip + `","allowed_countries":` + countries + `}`)
}

func (s *scenario) post(body string) *httptest.ResponseRecorder {
	return s.do(http.MethodPost, "/v1/ip-checks", "application/json", body)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", rec.Body, err)
	}
	return m
}

func country(code, name string) map[string]any {
	return map[string]any{"iso_code": code, "name": name}
}

const (
	textNoCountry   = "No country is recorded for this IP address in the geolocation database, so it is blocked."
	textPrivate     = "The IP address is in a private range and has no associated country, so it is blocked."
	textLoopback    = "The IP address is a loopback or link-local address and has no associated country, so it is blocked."
	textUnspecified = "The IP address is the unspecified address and has no associated country, so it is blocked."
	textReserved    = "The IP address is in a reserved range and has no associated country, so it is blocked."
)

func allowedText(code, name string) string {
	return "The IP address is located in " + code + " (" + name + "), which is in allowed_countries."
}

func blockedText(code, name string) string {
	return "The IP address is located in " + code + " (" + name + "), which is not in allowed_countries, so it is blocked."
}

func TestChecks(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		countries  string
		wantAllow  bool
		wantReason string
		wantDetail string
		wantCtry   any
	}{
		{"allowed", "81.2.69.160", `["GB"]`, true, "country_allowed", allowedText("GB", "United Kingdom"), country("GB", "United Kingdom")},
		{"not allowed, mixed case list", "81.2.69.160", `["de","FR"]`, false, "country_not_allowed", blockedText("GB", "United Kingdom"), country("GB", "United Kingdom")},
		{"IPv6 allowed", "2001:218::1", `["JP"]`, true, "country_allowed", allowedText("JP", "Japan"), country("JP", "Japan")},
		{"IPv6 not allowed", "2a02:cf40::1", `["GB"]`, false, "country_not_allowed", blockedText("NO", "Norway"), country("NO", "Norway")},
		{"IPv4-mapped IPv6", "::ffff:81.2.69.160", `["GB"]`, true, "country_allowed", allowedText("GB", "United Kingdom"), country("GB", "United Kingdom")},
		{"registered country is not used", "2.125.160.216", `["FR"]`, false, "country_not_allowed", blockedText("GB", "United Kingdom"), country("GB", "United Kingdom")},
		{"not in database", "8.8.8.8", `["GB"]`, false, "country_unknown", textNoCountry, nil},
		{"record without country", "214.1.1.1", `["GB"]`, false, "country_unknown", textNoCountry, nil},
		{"IPv6 record without country", "2a02:d500::1", `["GB"]`, false, "country_unknown", textNoCountry, nil},
		{"registered country only is not a fallback", "5.83.124.1", `["RO"]`, false, "country_unknown", textNoCountry, nil},
		{"private", "10.0.0.1", `["GB"]`, false, "country_unknown", textPrivate, nil},
		{"loopback", "127.0.0.1", `["GB"]`, false, "country_unknown", textLoopback, nil},
		{"link local", "169.254.1.1", `["GB"]`, false, "country_unknown", textLoopback, nil},
		{"link local with zone", "fe80::1%eth0", `["GB"]`, false, "country_unknown", textLoopback, nil},
		{"unspecified", "0.0.0.0", `["GB"]`, false, "country_unknown", textUnspecified, nil},
		{"reserved", "224.0.0.1", `["GB"]`, false, "country_unknown", textReserved, nil},
	}
	s := newScenario(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := s.check(tt.ip, tt.countries)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
			}
			want := map[string]any{"allowed": tt.wantAllow, "reason": tt.wantReason, "detail": tt.wantDetail, "country": tt.wantCtry}
			if got := decode(t, rec); !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
			}
		})
	}
}

func TestAcceptanceExampleBodyIsExact(t *testing.T) {
	rec := newScenario(t).check("81.2.69.160", `["GB"]`)
	const want = `{"allowed":true,"reason":"country_allowed","detail":"The IP address is located in GB (United Kingdom), which is in allowed_countries.","country":{"iso_code":"GB","name":"United Kingdom"}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestDuplicatesWarn(t *testing.T) {
	rec := newScenario(t).check("81.2.69.160", `["GB","gb","IT","IT"]`)
	body := decode(t, rec)
	if body["reason"] != "country_allowed" {
		t.Errorf("reason = %v, want country_allowed", body["reason"])
	}
	want := []any{
		map[string]any{"code": "duplicate_country", "pointer": "/allowed_countries/1", "detail": "'GB' appears more than once in allowed_countries."},
		map[string]any{"code": "duplicate_country", "pointer": "/allowed_countries/3", "detail": "'IT' appears more than once in allowed_countries."},
	}
	if !reflect.DeepEqual(body["warnings"], want) {
		t.Errorf("warnings = %v, want %v", body["warnings"], want)
	}
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []any
	}{
		{"empty list", `{"ip_address":"81.2.69.160","allowed_countries":[]}`, []any{
			map[string]any{"pointer": "/allowed_countries", "reason": "allowed_countries must contain at least one country code."},
		}},
		{"bad IP and bad country, in order", `{"ip_address":"999.1.1.1","allowed_countries":["USA"]}`, []any{
			map[string]any{"pointer": "/ip_address", "reason": "ip_address must be a valid IPv4 or IPv6 address."},
			map[string]any{"pointer": "/allowed_countries/0", "reason": "'USA' is not an ISO 3166-1 alpha-2 country code."},
		}},
		{"missing ip", `{"allowed_countries":["GB"]}`, []any{
			map[string]any{"pointer": "/ip_address", "reason": "ip_address is required."},
		}},
		{"not an object", `[]`, []any{
			map[string]any{"pointer": "", "reason": "The request body must be a JSON object."},
		}},
	}
	s := newScenario(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := s.post(tt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", got)
			}
			if got := decode(t, rec)["invalid_params"]; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("invalid_params = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBodyCap(t *testing.T) {
	const base = `{"ip_address":"81.2.69.160","allowed_countries":["GB"]}`
	s := newScenario(t)
	for size, want := range map[int]int{16384: http.StatusOK, 16385: http.StatusRequestEntityTooLarge} {
		if rec := s.post(base + strings.Repeat(" ", size-len(base))); rec.Code != want {
			t.Errorf("body of %d bytes: status = %d, want %d", size, rec.Code, want)
		}
	}
}

func TestLogsNeverContainAnIP(t *testing.T) {
	s := newScenario(t)
	for _, ip := range fixtureIPs {
		s.check(ip, `["GB"]`)
		s.check(ip, `["USA"]`)
		s.do(http.MethodGet, "/v1/ip-checks", "", "")
	}
	s.post(`{"ip_address":"8.8.8.8"}`)
	for _, ip := range fixtureIPs {
		if strings.Contains(s.logs.String(), ip) {
			t.Errorf("logs contain %s:\n%s", ip, s.logs)
		}
	}
}
