package netclass_test

import (
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "plain IPv4", in: "81.2.69.160", want: "81.2.69.160"},
		{name: "IPv6", in: "2001:218::1", want: "2001:218::1"},
		{name: "IPv4-mapped IPv6 is unmapped", in: "::ffff:81.2.69.160", want: "81.2.69.160"},
		{name: "zone is dropped", in: "fe80::1%eth0", want: "fe80::1"},
		{name: "leading zero in IPv4", in: "010.0.0.1", wantErr: true},
		{name: "leading space", in: " 1.2.3.4", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "octet out of range", in: "999.1.1.1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := netclass.Parse(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want an error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}
