package geoipdb

import (
	"net/netip"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

func BenchmarkLookupIPv4Hit(b *testing.B) { benchmarkLookup(b, "81.2.69.160") }

func BenchmarkLookupIPv6Hit(b *testing.B) { benchmarkLookup(b, "2001:218::1") }

func BenchmarkLookupMiss(b *testing.B) { benchmarkLookup(b, "8.8.8.8") }

func BenchmarkLookupParallel(b *testing.B) {
	db := openTestDB(b)
	addr := netip.MustParseAddr("81.2.69.160")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lookupAndDecode(b, db, addr)
		}
	})
}

func benchmarkLookup(b *testing.B, ip string) {
	db := openTestDB(b)
	addr := netip.MustParseAddr(ip)
	b.ReportAllocs()
	for b.Loop() {
		lookupAndDecode(b, db, addr)
	}
}

func lookupAndDecode(b *testing.B, db *maxminddb.Reader, addr netip.Addr) {
	res := db.Lookup(addr)
	if !res.Found() {
		return
	}
	var rec countryRecord
	if err := res.Decode(&rec); err != nil {
		b.Error(err)
	}
}
