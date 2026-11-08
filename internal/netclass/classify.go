package netclass

import "net/netip"

// Category tells what kind of address an IP is.
type Category string

// The address categories.
const (
	CategoryPrivate     Category = "private"
	CategoryLoopback    Category = "loopback"
	CategoryLinkLocal   Category = "link_local"
	CategoryUnspecified Category = "unspecified"
	CategoryReserved    Category = "reserved"
	CategoryPublic      Category = "public"
)

// reservedPrefixes are special-use ranges not covered by the net/netip checks.
var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// Classify returns the category of addr. The invalid zero address is reserved,
// so it can never count as public.
func Classify(addr netip.Addr) Category {
	addr = addr.Unmap()
	switch {
	case !addr.IsValid():
		return CategoryReserved
	case addr.IsUnspecified():
		return CategoryUnspecified
	case addr.IsLoopback():
		return CategoryLoopback
	case addr.IsLinkLocalUnicast():
		return CategoryLinkLocal
	case addr.IsPrivate():
		return CategoryPrivate
	case addr.IsMulticast() || inReservedPrefix(addr):
		return CategoryReserved
	default:
		return CategoryPublic
	}
}

func inReservedPrefix(addr netip.Addr) bool {
	for _, p := range reservedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
