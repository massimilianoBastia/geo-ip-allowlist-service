package netclass

import "net/netip"

// Category tells what kind of address an IP is.
type Category string

// The address categories. A public address may still be missing from the
// geolocation database; that case is decided elsewhere.
const (
	CategoryPrivate     Category = "private"
	CategoryLoopback    Category = "loopback"
	CategoryLinkLocal   Category = "link_local"
	CategoryUnspecified Category = "unspecified"
	CategoryReserved    Category = "reserved"
	CategoryPublic      Category = "public"
)

// Classify returns the category of addr. An IPv4-mapped IPv6 address is
// classified as the IPv4 address it wraps.
func Classify(addr netip.Addr) Category {
	return ""
}
