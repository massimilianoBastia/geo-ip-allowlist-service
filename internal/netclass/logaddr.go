package netclass

import "net/netip"

// MaskedAddr wraps an address so that logging it shows only the network
// prefix (/24 for IPv4, /48 for IPv6), never the full address. It is a safety
// net for a log line that names an address by mistake.
type MaskedAddr struct {
	addr netip.Addr
}

// NewMaskedAddr returns a MaskedAddr for addr.
func NewMaskedAddr(addr netip.Addr) MaskedAddr {
	return MaskedAddr{addr: addr}
}

// String returns the prefix of the address, or "invalid" for the zero address.
func (m MaskedAddr) String() string {
	return "not implemented"
}
