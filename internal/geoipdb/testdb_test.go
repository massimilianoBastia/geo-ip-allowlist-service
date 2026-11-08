package geoipdb

import (
	"os"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
)

const testDBPath = "../../testdata/GeoIP2-Country-Test.mmdb"

// openTestDB opens the committed synthetic database. OpenBytes keeps the
// slice, so it is never modified after this call.
func openTestDB(t testing.TB) *maxminddb.Reader {
	t.Helper()
	data, err := os.ReadFile(testDBPath)
	if err != nil {
		t.Fatalf("read test database: %v", err)
	}
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return db
}
