package request_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/request"
)

// FuzzParseInvariants checks the public parser contract rather than matching
// its implementation: failures have a documented type; accepted requests
// always have a normalized address and a nonempty set of assigned codes.
func FuzzParseInvariants(f *testing.F) {
	for _, seed := range []string{
		`{"ip_address":"81.2.69.160","allowed_countries":["gb","GB"]}`,
		`{"ip_address":"::ffff:81.2.69.160","allowed_countries":["GB"]}`,
		`{"ip_address":"fe80::1%eth0","allowed_countries":["JP"],"session_id":{}}`,
		`{"ip_address":null,"allowed_countries":[1,"XK",null]}`,
		`{"ip_address":"bad","ip_address":"2001:218::1","allowed_countries":["JP"]}`,
		`{"ip_address":"81.2.69.160","allowed_countries":["GB"],"requested_at":true}`,
		`[]`, `null`, `{} {}`, ``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		r, err := request.Parse(body)
		if !json.Valid(body) {
			if !errors.Is(err, request.ErrMalformed) {
				t.Fatalf("invalid JSON returned %v", err)
			}
			return
		}
		if err != nil {
			var invalid *request.InvalidError
			if !errors.As(err, &invalid) || len(invalid.Params) == 0 {
				t.Fatalf("valid JSON returned undocumented error: %v", err)
			}
			return
		}
		if !r.Addr.IsValid() || r.Addr.Is4In6() || r.Addr.Zone() != "" || len(r.Countries) == 0 {
			t.Fatalf("accepted request violates address/country invariants: %+v", r)
		}
		seen := make(map[string]bool)
		for _, code := range r.Countries {
			if !iso.Valid(code) || seen[code] {
				t.Fatalf("invalid or duplicate normalized code %q", code)
			}
			seen[code] = true
		}
	})
}
