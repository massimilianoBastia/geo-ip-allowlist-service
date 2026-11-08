package geoipdb

import (
	"errors"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenTestDatabase(t *testing.T) {
	db, err := Open(testDBPath)
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	info := db.Info()
	if info.Type != "GeoIP2-Country" {
		t.Errorf("Type = %q, want GeoIP2-Country", info.Type)
	}
	if info.BuiltAt.IsZero() {
		t.Error("BuiltAt is zero")
	}
}

func TestLookup(t *testing.T) {
	db, err := Open(testDBPath)
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	tests := []struct {
		ip   string
		want Record
	}{
		{"81.2.69.160", Record{Found: true, ISOCode: "GB", Name: "United Kingdom"}},
		{"2001:218::1", Record{Found: true, ISOCode: "JP", Name: "Japan"}},
		{"2.125.160.216", Record{Found: true, ISOCode: "GB", Name: "United Kingdom"}},
		{"214.1.1.1", Record{Found: true}},
		{"5.83.124.1", Record{Found: true}},
		{"8.8.8.8", Record{}},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got, err := db.Lookup(netip.MustParseAddr(tt.ip))
			if err != nil {
				t.Fatalf("Lookup error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Lookup = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Open(filepath.Join(t.TempDir(), "missing.mmdb"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Open error = %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("not a database", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeros.mmdb")
		if err := os.WriteFile(path, make([]byte, 100), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); err == nil {
			t.Error("Open of 100 zero bytes succeeded, want an error")
		}
	})
}

func TestCheckType(t *testing.T) {
	tests := []struct {
		typ     string
		wantErr bool
	}{
		{"GeoLite2-Country", false},
		{"GeoIP2-Country", false},
		{"GeoLite2-City", true},
		{"GeoLite2-ASN", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			if err := CheckType(tt.typ); (err != nil) != tt.wantErr {
				t.Errorf("CheckType(%q) error = %v, wantErr %v", tt.typ, err, tt.wantErr)
			}
		})
	}
}
