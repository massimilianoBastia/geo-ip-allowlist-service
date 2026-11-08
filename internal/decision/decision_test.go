package decision_test

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/decision"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

type fakeLookup struct {
	record geoipdb.Record
	err    error
	calls  int
}

func (f *fakeLookup) Lookup(netip.Addr) (geoipdb.Record, error) {
	f.calls++
	return f.record, f.err
}

func found(code, name string) geoipdb.Record {
	return geoipdb.Record{Found: true, ISOCode: code, Name: name}
}

const (
	textAllowed     = "The IP address is located in GB (United Kingdom), which is in allowed_countries."
	textNotAllowed  = "The IP address is located in GB (United Kingdom), which is not in allowed_countries, so it is blocked."
	textPrivate     = "The IP address is in a private range and has no associated country, so it is blocked."
	textLoopback    = "The IP address is a loopback or link-local address and has no associated country, so it is blocked."
	textUnspecified = "The IP address is the unspecified address and has no associated country, so it is blocked."
	textReserved    = "The IP address is in a reserved range and has no associated country, so it is blocked."
	textUnmapped    = "No country is recorded for this IP address in the geolocation database, so it is blocked."
	textUnsupported = "The IP address is located in a country whose code is not an ISO 3166-1 alpha-2 code, so it cannot be in any allowed list and is blocked."
)

func TestDecide(t *testing.T) {
	gb := &decision.Country{ISOCode: "GB", Name: "United Kingdom"}
	tests := []struct {
		name        string
		ip          string
		allowed     []string
		record      geoipdb.Record
		want        decision.Result
		wantLookups int
	}{
		{"allowed", "81.2.69.160", []string{"GB"}, found("GB", "United Kingdom"),
			decision.Result{Allowed: true, Reason: decision.ReasonAllowed, Detail: textAllowed, Country: gb}, 1},
		{"not allowed", "81.2.69.160", []string{"DE"}, found("GB", "United Kingdom"),
			decision.Result{Reason: decision.ReasonNotAllowed, Detail: textNotAllowed, Country: gb}, 1},
		{"allowed, empty name", "81.2.69.160", []string{"GB"}, found("GB", ""),
			decision.Result{Allowed: true, Reason: decision.ReasonAllowed, Detail: "The IP address is located in GB, which is in allowed_countries.", Country: &decision.Country{ISOCode: "GB"}}, 1},
		{"not allowed, empty name", "81.2.69.160", []string{"DE"}, found("GB", ""),
			decision.Result{Reason: decision.ReasonNotAllowed, Detail: "The IP address is located in GB, which is not in allowed_countries, so it is blocked.", Country: &decision.Country{ISOCode: "GB"}}, 1},
		{"private", "10.0.0.1", []string{"GB"}, found("GB", "x"),
			decision.Result{Reason: decision.ReasonUnknown, Detail: textPrivate, Category: netclass.CategoryPrivate}, 0},
		{"loopback", "127.0.0.1", []string{"GB"}, found("GB", "x"),
			decision.Result{Reason: decision.ReasonUnknown, Detail: textLoopback, Category: netclass.CategoryLoopback}, 0},
		{"link local", "fe80::1", []string{"GB"}, found("GB", "x"),
			decision.Result{Reason: decision.ReasonUnknown, Detail: textLoopback, Category: netclass.CategoryLinkLocal}, 0},
		{"unspecified", "0.0.0.0", []string{"GB"}, found("GB", "x"),
			decision.Result{Reason: decision.ReasonUnknown, Detail: textUnspecified, Category: netclass.CategoryUnspecified}, 0},
		{"reserved", "224.0.0.1", []string{"GB"}, found("GB", "x"),
			decision.Result{Reason: decision.ReasonUnknown, Detail: textReserved, Category: netclass.CategoryReserved}, 0},
		{"not found", "8.8.8.8", []string{"GB"}, geoipdb.Record{},
			decision.Result{Reason: decision.ReasonUnknown, Detail: textUnmapped, Category: decision.CategoryPublicUnmapped}, 1},
		{"record without country", "214.1.1.1", []string{"GB"}, geoipdb.Record{Found: true},
			decision.Result{Reason: decision.ReasonUnknown, Detail: textUnmapped, Category: decision.CategoryPublicUnmapped}, 1},
		{"non-ISO code", "81.2.69.160", []string{"GB"}, found("XK", "Kosovo"),
			decision.Result{Reason: decision.ReasonUnsupported, Detail: textUnsupported, Country: &decision.Country{ISOCode: "XK", Name: "Kosovo"}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLookup{record: tt.record}
			got, err := decision.Decide(fake, netip.MustParseAddr(tt.ip), tt.allowed)
			if err != nil {
				t.Fatalf("Decide error = %v", err)
			}
			if got.Allowed != tt.want.Allowed || got.Reason != tt.want.Reason ||
				got.Detail != tt.want.Detail || got.Category != tt.want.Category {
				t.Errorf("Decide = %+v, want %+v", got, tt.want)
			}
			if (got.Country == nil) != (tt.want.Country == nil) || (got.Country != nil && *got.Country != *tt.want.Country) {
				t.Errorf("Country = %+v, want %+v", got.Country, tt.want.Country)
			}
			if fake.calls != tt.wantLookups {
				t.Errorf("lookups = %d, want %d", fake.calls, tt.wantLookups)
			}
		})
	}
}

func TestDecideLookupError(t *testing.T) {
	boom := errors.New("boom")
	_, err := decision.Decide(&fakeLookup{err: boom}, netip.MustParseAddr("81.2.69.160"), []string{"GB"})
	if !errors.Is(err, boom) {
		t.Errorf("Decide error = %v, want it to wrap %v", err, boom)
	}
}
