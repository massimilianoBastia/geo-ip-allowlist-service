package netclass_test

import (
	"net/netip"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		want  netclass.Category
		addrs []string
	}{
		{netclass.CategoryPrivate, []string{"10.0.0.1", "172.16.0.1", "192.168.1.1", "fd00::1", "::ffff:10.0.0.1"}},
		{netclass.CategoryLoopback, []string{"127.0.0.1", "::1"}},
		{netclass.CategoryLinkLocal, []string{"169.254.1.1", "fe80::1"}},
		{netclass.CategoryUnspecified, []string{"0.0.0.0", "::"}},
		{netclass.CategoryReserved, []string{
			"224.0.0.1", "ff02::1", "100.64.0.1", "192.0.2.1", "198.51.100.1",
			"203.0.113.1", "2001:db8::1", "240.0.0.1", "255.255.255.255",
			"0.1.2.3", "198.18.0.1", "192.0.0.1",
		}},
		{netclass.CategoryPublic, []string{"81.2.69.160", "8.8.8.8", "2001:218::1", "172.32.0.1", "100.128.0.1"}},
	}
	for _, tt := range tests {
		for _, s := range tt.addrs {
			t.Run(string(tt.want)+"/"+s, func(t *testing.T) {
				addr := netip.MustParseAddr(s)
				if got := netclass.Classify(addr); got != tt.want {
					t.Errorf("Classify(%s) = %q, want %q", s, got, tt.want)
				}
			})
		}
	}
}
