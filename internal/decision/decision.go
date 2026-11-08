// Package decision decides whether an IP address is in an allowed country.
package decision

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

// Reason is the stable machine-readable outcome of a check.
type Reason string

// The four reasons.
const (
	ReasonAllowed     Reason = "country_allowed"
	ReasonNotAllowed  Reason = "country_not_allowed"
	ReasonUnknown     Reason = "country_unknown"
	ReasonUnsupported Reason = "country_unsupported"
)

// CategoryPublicUnmapped is the category of a public address with no country.
const CategoryPublicUnmapped netclass.Category = "public_unmapped"

// Country is the country an address was found in.
type Country struct {
	ISOCode string `json:"iso_code"`
	Name    string `json:"name"`
}

// Lookuper finds the country recorded for an address.
type Lookuper interface {
	Lookup(addr netip.Addr) (geoipdb.Record, error)
}

// Result is the outcome of a check. Category is set only for ReasonUnknown.
type Result struct {
	Allowed  bool
	Reason   Reason
	Detail   string
	Country  *Country
	Category netclass.Category
}

// Decide checks addr against the allowed country codes. Special-use addresses
// are never looked up.
func Decide(l Lookuper, addr netip.Addr, allowed []string) (Result, error) {
	if category := netclass.Classify(addr); category != netclass.CategoryPublic {
		return unknown(category), nil
	}
	record, err := l.Lookup(addr)
	if err != nil {
		return Result{}, fmt.Errorf("look up country: %w", err)
	}
	if !record.Found || record.ISOCode == "" {
		return unknown(CategoryPublicUnmapped), nil
	}

	country := &Country{ISOCode: record.ISOCode, Name: record.Name}
	label := "The IP address is located in " + country.ISOCode
	if country.Name != "" {
		label += " (" + country.Name + ")"
	}
	switch {
	case !iso.Valid(country.ISOCode):
		return Result{Reason: ReasonUnsupported, Country: country, Detail: "The IP address is located in a country whose code is not an ISO 3166-1 alpha-2 code, so it cannot be in any allowed list and is blocked."}, nil
	case slices.Contains(allowed, country.ISOCode):
		return Result{Allowed: true, Reason: ReasonAllowed, Country: country, Detail: label + ", which is in allowed_countries."}, nil
	default:
		return Result{Reason: ReasonNotAllowed, Country: country, Detail: label + ", which is not in allowed_countries, so it is blocked."}, nil
	}
}

func unknown(category netclass.Category) Result {
	return Result{Reason: ReasonUnknown, Category: category, Detail: unknownDetail(category)}
}

func unknownDetail(category netclass.Category) string {
	switch category {
	case netclass.CategoryPrivate:
		return "The IP address is in a private range and has no associated country, so it is blocked."
	case netclass.CategoryLoopback, netclass.CategoryLinkLocal:
		return "The IP address is a loopback or link-local address and has no associated country, so it is blocked."
	case netclass.CategoryUnspecified:
		return "The IP address is the unspecified address and has no associated country, so it is blocked."
	case CategoryPublicUnmapped:
		return "No country is recorded for this IP address in the geolocation database, so it is blocked."
	default:
		return "The IP address is in a reserved range and has no associated country, so it is blocked."
	}
}
