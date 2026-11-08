package request_test

import (
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

const wantIPReason = "ip_address must be a valid IPv4 or IPv6 address."

func param(pointer, reason string) request.InvalidParam {
	return request.InvalidParam{Pointer: pointer, Reason: reason}
}

func notISO(pointer, entry string) request.InvalidParam {
	return param(pointer, "'"+entry+"' is not an ISO 3166-1 alpha-2 country code.")
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []request.InvalidParam
	}{
		{"empty object", `{}`, []request.InvalidParam{
			param("/ip_address", "ip_address is required."),
			param("/allowed_countries", "allowed_countries is required."),
		}},
		{"both null", `{"ip_address":null,"allowed_countries":null}`, []request.InvalidParam{
			param("/ip_address", "ip_address is required."),
			param("/allowed_countries", "allowed_countries is required."),
		}},
		{"wrong types", `{"ip_address":123,"allowed_countries":"GB"}`, []request.InvalidParam{
			param("/ip_address", "ip_address must be a string."),
			param("/allowed_countries", "allowed_countries must be an array of strings."),
		}},
		{"empty list", `{"ip_address":"81.2.69.160","allowed_countries":[]}`, []request.InvalidParam{
			param("/allowed_countries", "allowed_countries must contain at least one country code."),
		}},
		{"leading zero", `{"ip_address":"010.0.0.1","allowed_countries":["GB"]}`, []request.InvalidParam{param("/ip_address", wantIPReason)}},
		{"leading space", `{"ip_address":" 1.2.3.4","allowed_countries":["GB"]}`, []request.InvalidParam{param("/ip_address", wantIPReason)}},
		{"empty string", `{"ip_address":"","allowed_countries":["GB"]}`, []request.InvalidParam{param("/ip_address", wantIPReason)}},
		{"octet out of range", `{"ip_address":"999.1.1.1","allowed_countries":["GB"]}`, []request.InvalidParam{param("/ip_address", wantIPReason)}},
		{"non-string entries", `{"ip_address":"81.2.69.160","allowed_countries":[1,"GB",null]}`, []request.InvalidParam{
			param("/allowed_countries/0", "Each entry in allowed_countries must be a string."),
			param("/allowed_countries/2", "Each entry in allowed_countries must be a string."),
		}},
		{"non-ISO entries quoted as sent", `{"ip_address":"81.2.69.160","allowed_countries":["USA","UK","1"," DE","ıt","XK"]}`, []request.InvalidParam{
			notISO("/allowed_countries/0", "USA"),
			notISO("/allowed_countries/1", "UK"),
			notISO("/allowed_countries/2", "1"),
			notISO("/allowed_countries/3", " DE"),
			notISO("/allowed_countries/4", "ıt"),
			notISO("/allowed_countries/5", "XK"),
		}},
		{"ip first then countries", `{"ip_address":"999.1.1.1","allowed_countries":["USA"]}`, []request.InvalidParam{
			param("/ip_address", wantIPReason),
			notISO("/allowed_countries/0", "USA"),
		}},
		{"misspelled ip_address is missing", `{"ip_adress":"81.2.69.160","allowed_countries":["GB"]}`, []request.InvalidParam{
			param("/ip_address", "ip_address is required."),
		}},
		{"array body", `[]`, []request.InvalidParam{param("", "The request body must be a JSON object.")}},
		{"null body", `null`, []request.InvalidParam{param("", "The request body must be a JSON object.")}},
		{"string body", `"x"`, []request.InvalidParam{param("", "The request body must be a JSON object.")}},
		{"number body", `42`, []request.InvalidParam{param("", "The request body must be a JSON object.")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := request.Parse([]byte(tt.body))
			var invalid *request.InvalidError
			if !errors.As(err, &invalid) {
				t.Fatalf("Parse error = %v, want *InvalidError", err)
			}
			if !reflect.DeepEqual(invalid.Params, tt.want) {
				t.Errorf("params = %+v, want %+v", invalid.Params, tt.want)
			}
		})
	}
}

func TestParseMalformed(t *testing.T) {
	for _, body := range []string{``, `not json`, `{"a":1} {}`} {
		t.Run(body, func(t *testing.T) {
			if _, err := request.Parse([]byte(body)); !errors.Is(err, request.ErrMalformed) {
				t.Errorf("Parse(%q) error = %v, want ErrMalformed", body, err)
			}
		})
	}
}

func TestParseValid(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantAddr      string
		wantCountries []string
		wantWarnings  []request.Warning
		wantUnknown   []string
	}{
		{
			name: "lower case and duplicate", body: `{"ip_address":"81.2.69.160","allowed_countries":["gb","IT","GB"]}`,
			wantAddr: "81.2.69.160", wantCountries: []string{"GB", "IT"},
			wantWarnings: []request.Warning{{Code: "duplicate_country", Pointer: "/allowed_countries/2", Detail: "'GB' appears more than once in allowed_countries."}},
		},
		{
			name: "duplicate after case folding", body: `{"ip_address":"81.2.69.160","allowed_countries":["IT","DE","it"]}`,
			wantAddr: "81.2.69.160", wantCountries: []string{"IT", "DE"},
			wantWarnings: []request.Warning{{Code: "duplicate_country", Pointer: "/allowed_countries/2", Detail: "'IT' appears more than once in allowed_countries."}},
		},
		{name: "zone dropped", body: `{"ip_address":"fe80::1%eth0","allowed_countries":["GB"]}`, wantAddr: "fe80::1", wantCountries: []string{"GB"}},
		{name: "IPv4-mapped unmapped", body: `{"ip_address":"::ffff:81.2.69.160","allowed_countries":["GB"]}`, wantAddr: "81.2.69.160", wantCountries: []string{"GB"}},
		{name: "unknown field", body: `{"ip_adress":"1.1.1.1","ip_address":"81.2.69.160","allowed_countries":["GB"],"foo":1}`,
			wantAddr: "81.2.69.160", wantCountries: []string{"GB"}, wantUnknown: []string{"foo", "ip_adress"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := request.Parse([]byte(tt.body))
			if err != nil {
				t.Fatalf("Parse error = %v", err)
			}
			if got.Addr != netip.MustParseAddr(tt.wantAddr) {
				t.Errorf("Addr = %v, want %v", got.Addr, tt.wantAddr)
			}
			if !reflect.DeepEqual(got.Countries, tt.wantCountries) {
				t.Errorf("Countries = %v, want %v", got.Countries, tt.wantCountries)
			}
			if !reflect.DeepEqual(got.Warnings, tt.wantWarnings) {
				t.Errorf("Warnings = %+v, want %+v", got.Warnings, tt.wantWarnings)
			}
			if !reflect.DeepEqual(got.UnknownFields, tt.wantUnknown) {
				t.Errorf("UnknownFields = %v, want %v", got.UnknownFields, tt.wantUnknown)
			}
		})
	}
}

func TestParseOptionalFields(t *testing.T) {
	const sessionWarn = "session_id must be a string of at most 128 characters; the value was ignored."
	const timeWarn = "requested_at must be a valid RFC 3339 timestamp; the value was ignored."
	tests := []struct {
		name         string
		extra        string
		wantSession  string
		wantTime     string
		wantWarnings []request.Warning
		wantIgnored  []request.IgnoredField
	}{
		{name: "128 runes kept", extra: `"session_id":"` + strings.Repeat("é", 128) + `"`, wantSession: strings.Repeat("é", 128)},
		{
			name: "129 runes ignored", extra: `"session_id":"` + strings.Repeat("é", 129) + `"`,
			wantWarnings: []request.Warning{{Code: "invalid_session_id", Pointer: "/session_id", Detail: sessionWarn}},
			wantIgnored:  []request.IgnoredField{{Name: "session_id", Length: 129}},
		},
		{
			name: "number session_id", extra: `"session_id":42`,
			wantWarnings: []request.Warning{{Code: "invalid_session_id", Pointer: "/session_id", Detail: sessionWarn}},
			wantIgnored:  []request.IgnoredField{{Name: "session_id", JSONType: "number"}},
		},
		{name: "empty session_id is absent", extra: `"session_id":""`},
		{name: "fractional seconds with offset kept", extra: `"requested_at":"2026-11-08T10:00:00.5+01:00"`, wantTime: "2026-11-08T10:00:00.5+01:00"},
		{
			name: "invalid requested_at", extra: `"requested_at":"yesterday"`,
			wantWarnings: []request.Warning{{Code: "invalid_requested_at", Pointer: "/requested_at", Detail: timeWarn}},
			wantIgnored:  []request.IgnoredField{{Name: "requested_at", Length: 9}},
		},
		{
			name: "number requested_at", extra: `"requested_at":5`,
			wantWarnings: []request.Warning{{Code: "invalid_requested_at", Pointer: "/requested_at", Detail: timeWarn}},
			wantIgnored:  []request.IgnoredField{{Name: "requested_at", JSONType: "number"}},
		},
		{
			name:  "warnings ordered: duplicates, session_id, requested_at",
			extra: `"requested_at":"x","session_id":false,"allowed_countries":["GB","GB"]`,
			wantWarnings: []request.Warning{
				{Code: "duplicate_country", Pointer: "/allowed_countries/1", Detail: "'GB' appears more than once in allowed_countries."},
				{Code: "invalid_session_id", Pointer: "/session_id", Detail: sessionWarn},
				{Code: "invalid_requested_at", Pointer: "/requested_at", Detail: timeWarn},
			},
			wantIgnored: []request.IgnoredField{{Name: "session_id", JSONType: "boolean"}, {Name: "requested_at", Length: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"ip_address":"81.2.69.160",` + tt.extra
			if !strings.Contains(tt.extra, "allowed_countries") {
				body += `,"allowed_countries":["GB"]`
			}
			got, err := request.Parse([]byte(body + `}`))
			if err != nil {
				t.Fatalf("Parse error = %v", err)
			}
			if got.SessionID != tt.wantSession {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.wantSession)
			}
			if got.RequestedAt != tt.wantTime {
				t.Errorf("RequestedAt = %q, want %q", got.RequestedAt, tt.wantTime)
			}
			if !reflect.DeepEqual(got.Warnings, tt.wantWarnings) {
				t.Errorf("Warnings = %+v, want %+v", got.Warnings, tt.wantWarnings)
			}
			if !reflect.DeepEqual(got.Ignored, tt.wantIgnored) {
				t.Errorf("Ignored = %+v, want %+v", got.Ignored, tt.wantIgnored)
			}
		})
	}
}

// TestParseNullAndEmptyOptionalFields covers G60: a JSON null is handled like
// any other value that is not a string, and an empty requested_at is invalid.
func TestParseNullAndEmptyOptionalFields(t *testing.T) {
	const sessionWarn = "session_id must be a string of at most 128 characters; the value was ignored."
	const timeWarn = "requested_at must be a valid RFC 3339 timestamp; the value was ignored."
	tests := []struct {
		name         string
		extra        string
		wantWarnings []request.Warning
		wantIgnored  []request.IgnoredField
	}{
		{"null session_id", `"session_id":null`,
			[]request.Warning{{Code: "invalid_session_id", Pointer: "/session_id", Detail: sessionWarn}},
			[]request.IgnoredField{{Name: "session_id", JSONType: "null"}}},
		{"null requested_at", `"requested_at":null`,
			[]request.Warning{{Code: "invalid_requested_at", Pointer: "/requested_at", Detail: timeWarn}},
			[]request.IgnoredField{{Name: "requested_at", JSONType: "null"}}},
		{"empty requested_at", `"requested_at":""`,
			[]request.Warning{{Code: "invalid_requested_at", Pointer: "/requested_at", Detail: timeWarn}},
			[]request.IgnoredField{{Name: "requested_at", Length: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := request.Parse([]byte(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],` + tt.extra + `}`))
			if err != nil {
				t.Fatalf("Parse error = %v", err)
			}
			if got.SessionID != "" || got.RequestedAt != "" {
				t.Errorf("SessionID/RequestedAt = %q/%q, want both empty", got.SessionID, got.RequestedAt)
			}
			if !reflect.DeepEqual(got.Warnings, tt.wantWarnings) {
				t.Errorf("Warnings = %+v, want %+v", got.Warnings, tt.wantWarnings)
			}
			if !reflect.DeepEqual(got.Ignored, tt.wantIgnored) {
				t.Errorf("Ignored = %+v, want %+v", got.Ignored, tt.wantIgnored)
			}
		})
	}
}

func TestParseLargeNumbers(t *testing.T) {
	tests := []struct {
		name, body string
		invalid    bool
		ignored    int
	}{
		{"unknown field", `{"ip_address":"81.2.69.160","allowed_countries":["GB"],"ignored":1e400}`, false, 0},
		{"optional fields", `{"ip_address":"81.2.69.160","allowed_countries":["GB"],"session_id":1e400,"requested_at":-1e400}`, false, 2},
		{"required field", `{"ip_address":1e400,"allowed_countries":["GB"]}`, true, 0},
		{"top level", `1e400`, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := request.Parse([]byte(tt.body))
			if tt.invalid {
				var invalid *request.InvalidError
				if !errors.As(err, &invalid) {
					t.Fatalf("error = %v, want validation error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if req.Addr.String() != "81.2.69.160" || !reflect.DeepEqual(req.Countries, []string{"GB"}) {
				t.Fatalf("unexpected parsed request: %+v", req)
			}
			if len(req.Ignored) != tt.ignored {
				t.Fatalf("ignored fields = %v", req.Ignored)
			}
			for _, field := range req.Ignored {
				if field.JSONType != "number" {
					t.Errorf("ignored type = %q, want number", field.JSONType)
				}
			}
		})
	}
}
