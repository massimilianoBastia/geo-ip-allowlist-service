package netclass

import (
	"log/slog"
	"net/netip"
)

const (
	maskedBitsIPv4 = 24
	maskedBitsIPv6 = 48
)

// MaskedAddr logs only the network prefix (/24 IPv4, /48 IPv6), as a safety
// net against a log line that names a full address by mistake.
type MaskedAddr struct {
	addr netip.Addr
}

// NewMaskedAddr returns a MaskedAddr for addr.
func NewMaskedAddr(addr netip.Addr) MaskedAddr {
	return MaskedAddr{addr: addr}
}

// String returns the prefix of the address, or "invalid" for the zero address.
func (m MaskedAddr) String() string {
	if !m.addr.IsValid() {
		return "invalid"
	}
	addr := m.addr.WithZone("").Unmap()
	bits := maskedBitsIPv6
	if addr.Is4() {
		bits = maskedBitsIPv4
	}
	return netip.PrefixFrom(addr, bits).Masked().String()
}

// LogValue implements slog.LogValuer.
func (m MaskedAddr) LogValue() slog.Value {
	return slog.StringValue(m.String())
}
