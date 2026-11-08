package request_test

import (
	"fmt"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

// TestParseRequestedAtRFC3339Profile checks that only strict RFC 3339
// timestamps are kept; Go's own parser also accepts one-digit hours and
// out-of-range offsets, which the contract does not.
func TestParseRequestedAtRFC3339Profile(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"2026-11-08T10:00:00Z", true},
		{"2026-11-08T10:00:00.123456789Z", true},
		{"2026-11-08T10:00:00+23:59", true},
		{"2026-11-08T10:00:00-00:00", true},
		{"2026-11-08T1:00:00Z", false},
		{"2026-11-08T10:0:00Z", false},
		{"2026-11-08T10:00:0Z", false},
		{"2026-11-08T10:00:00+24:00", false},
		{"2026-11-08T10:00:00+23:60", false},
		{"2026-11-08T10:00:00+1:00", false},
		{"2026-11-08T10:00:00+0100", false},
		{"2026-11-08T10:00:00", false},
		{"2026-11-08 10:00:00Z", false},
		{"2026-11-08T10:00:00.Z", false},
		{"2026-11-08T24:00:00Z", false},
		{"2026-02-30T10:00:00Z", false},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			body := fmt.Sprintf(`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"requested_at":%q}`, tc.value)
			r, err := request.Parse([]byte(body))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if tc.valid {
				if r.RequestedAt != tc.value || len(r.Warnings) != 0 {
					t.Fatalf("got RequestedAt %q, warnings %v; want kept and no warning", r.RequestedAt, r.Warnings)
				}
				return
			}
			if r.RequestedAt != "" || len(r.Warnings) != 1 || r.Warnings[0].Code != "invalid_requested_at" {
				t.Fatalf("got RequestedAt %q, warnings %v; want ignored with invalid_requested_at", r.RequestedAt, r.Warnings)
			}
		})
	}
}
