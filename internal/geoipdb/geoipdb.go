// Package geoipdb reads a MaxMind Country database into memory and looks up
// the country of an IP address.
package geoipdb

import (
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// Info describes a loaded database.
type Info struct {
	Type    string
	BuiltAt time.Time
}

// Record is the result of a lookup. Found is false when the address is not in
// the database; ISOCode is empty when the record has no country.
type Record struct {
	Found   bool
	ISOCode string
	Name    string
}

// SanityCheck is a known address and the country the database must give for it.
type SanityCheck struct {
	Addr    netip.Addr
	ISOCode string
}

// Database is an opened database that can be queried.
type Database interface {
	Info() Info
	Lookup(addr netip.Addr) (Record, error)
}

// DB is a Country database held in memory. It is safe for concurrent use.
type DB struct {
	reader *maxminddb.Reader
	info   Info
}

// record holds the only fields the service decodes.
type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
		Names   struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"country"`
}

// Open reads the database file at path.
func Open(path string) (*DB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read database file: %w", err)
	}
	return OpenBytes(data)
}

// OpenBytes opens a database held in data. The caller must not modify data
// afterwards.
func OpenBytes(data []byte) (*DB, error) {
	reader, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	meta := reader.Metadata
	return &DB{reader: reader, info: Info{Type: meta.DatabaseType, BuiltAt: meta.BuildTime().UTC()}}, nil
}

// Info returns the type and build time of the database.
func (d *DB) Info() Info { return d.info }

// Lookup returns the country recorded for addr.
func (d *DB) Lookup(addr netip.Addr) (Record, error) {
	result := d.reader.Lookup(addr)
	if err := result.Err(); err != nil {
		return Record{}, fmt.Errorf("look up address: %w", err)
	}
	if !result.Found() {
		return Record{}, nil
	}
	var rec record
	if err := result.Decode(&rec); err != nil {
		return Record{}, fmt.Errorf("decode record: %w", err)
	}
	return Record{Found: true, ISOCode: rec.Country.ISOCode, Name: rec.Country.Names.EN}, nil
}

// CheckType returns an error unless typ is a GeoLite2 or GeoIP2 Country database.
func CheckType(typ string) error {
	switch typ {
	case "GeoLite2-Country", "GeoIP2-Country":
		return nil
	}
	return fmt.Errorf("database type %q is not a Country database", typ)
}
