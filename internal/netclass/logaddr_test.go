package netclass_test

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

func TestMaskedAddrString(t *testing.T) {
	tests := []struct {
		name string
		addr netip.Addr
		want string
	}{
		{"IPv4 keeps /24", netip.MustParseAddr("81.2.69.160"), "81.2.69.0/24"},
		{"IPv6 keeps /48", netip.MustParseAddr("2001:218::1"), "2001:218::/48"},
		{"IPv4-mapped IPv6 renders as IPv4", netip.MustParseAddr("::ffff:81.2.69.160"), "81.2.69.0/24"},
		{"private IPv4", netip.MustParseAddr("10.1.2.3"), "10.1.2.0/24"},
		{"zero address", netip.Addr{}, "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := netclass.NewMaskedAddr(tt.addr).String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaskedAddrInLogRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Info("lookup", "address", netclass.NewMaskedAddr(netip.MustParseAddr("81.2.69.160")))

	out := buf.String()
	if !strings.Contains(out, `"81.2.69.0/24"`) {
		t.Errorf("log record %q does not contain the prefix %q", out, `"81.2.69.0/24"`)
	}
	if strings.Contains(out, "81.2.69.160") {
		t.Errorf("log record %q contains the full address", out)
	}
}
