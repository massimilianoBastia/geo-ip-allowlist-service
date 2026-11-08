// Package netclass parses and normalises IP addresses, classifies them by
// address category, and provides a log-safe address type.
package netclass

import (
	"fmt"
	"net/netip"
)

// Parse parses s as an IPv4 or IPv6 address. It drops an IPv6 zone and turns
// an IPv4-mapped IPv6 address into plain IPv4.
func Parse(s string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse IP address: %w", err)
	}
	return addr.WithZone("").Unmap(), nil
}
