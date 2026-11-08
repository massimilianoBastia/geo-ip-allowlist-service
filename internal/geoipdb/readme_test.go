package geoipdb

import (
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	readmePath  = "../../testdata/README.md"
	beginMarker = "<!-- BEGIN GENERATED -->"
	endMarker   = "<!-- END GENERATED -->"
)

var update = flag.Bool("update", false, "rewrite the generated block in testdata/README.md")

// fixtureAddresses are the literals the scenarios and other tests rely on.
var fixtureAddresses = []string{
	"81.2.69.160", "2.125.160.216", "2001:218::1", "2a02:cf40::1",
	"214.1.1.1", "2a02:d500::1", "5.83.124.1", "8.8.8.8",
}

// wantBlock is what the database must contain, written out by hand.
const wantBlock = beginMarker + `
Database type: GeoIP2-Country

| Address | Network | Country | Name | Registered country |
|---|---|---|---|---|
| ` + "`81.2.69.160` | `81.2.69.160/27`" + ` | GB | United Kingdom | US |
| ` + "`2.125.160.216` | `2.125.160.216/29`" + ` | GB | United Kingdom | FR |
| ` + "`2001:218::1` | `2001:218::/32`" + ` | JP | Japan | JP |
| ` + "`2a02:cf40::1` | `2a02:cf40::/29`" + ` | NO | Norway | NO |
| ` + "`214.1.1.1` | `214.1.1.0/24`" + ` | (none) | | (none) |
| ` + "`2a02:d500::1` | `2a02:d500::/29`" + ` | (none) | | (none) |
| ` + "`5.83.124.1` | `5.83.124.0/22`" + ` | (none) | | RO |
| ` + "`8.8.8.8`" + ` | not found | | | |

non-ISO country codes in database: 0
` + endMarker

type countryRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"registered_country"`
}

func TestDatabaseMatchesExpectedFacts(t *testing.T) {
	if got := generateBlock(t, openTestDB(t)); got != wantBlock {
		t.Errorf("database facts differ from the expected block\ngot:\n%s\nwant:\n%s", got, wantBlock)
	}
}

func TestReadmeBlockIsCurrent(t *testing.T) {
	want := generateBlock(t, openTestDB(t))
	readme := readFile(t, readmePath)

	if *update {
		writeFile(t, readmePath, replaceBlock(readme, want))
		return
	}
	got, ok := extractBlock(readme)
	if !ok {
		t.Fatalf("%s has no generated block; run: go test ./internal/geoipdb -run TestReadmeBlockIsCurrent -update", readmePath)
	}
	if got != want {
		t.Errorf("generated block in %s is out of date; run with -update\ngot:\n%s\nwant:\n%s", readmePath, got, want)
	}
}

func generateBlock(t *testing.T, db *maxminddb.Reader) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nDatabase type: %s\n\n", beginMarker, db.Metadata.DatabaseType)
	b.WriteString("| Address | Network | Country | Name | Registered country |\n|---|---|---|---|---|\n")
	for _, s := range fixtureAddresses {
		b.WriteString(fixtureRow(t, db, netip.MustParseAddr(s)) + "\n")
	}
	fmt.Fprintf(&b, "\nnon-ISO country codes in database: %d\n%s", countNonISO(t, db), endMarker)
	return b.String()
}

func fixtureRow(t *testing.T, db *maxminddb.Reader, addr netip.Addr) string {
	t.Helper()
	res := db.Lookup(addr)
	if err := res.Err(); err != nil {
		t.Fatalf("lookup %s: %v", addr, err)
	}
	if !res.Found() {
		return fmt.Sprintf("| `%s` | not found | | | |", addr)
	}
	var rec countryRecord
	if err := res.Decode(&rec); err != nil {
		t.Fatalf("decode %s: %v", addr, err)
	}
	registered := rec.RegisteredCountry.ISOCode
	if registered == "" {
		registered = "(none)"
	}
	if rec.Country.ISOCode == "" {
		return fmt.Sprintf("| `%s` | `%s` | (none) | | %s |", addr, res.Prefix(), registered)
	}
	return fmt.Sprintf("| `%s` | `%s` | %s | %s | %s |", addr, res.Prefix(), rec.Country.ISOCode, rec.Country.Names["en"], registered)
}

func countNonISO(t *testing.T, db *maxminddb.Reader) int {
	t.Helper()
	n := 0
	for res := range db.Networks() {
		var rec countryRecord
		if err := res.Decode(&rec); err != nil {
			t.Fatalf("decode %s: %v", res.Prefix(), err)
		}
		if code := rec.Country.ISOCode; code != "" && !iso.Valid(code) {
			n++
		}
	}
	return n
}

func extractBlock(readme string) (string, bool) {
	start := strings.Index(readme, beginMarker)
	end := strings.Index(readme, endMarker)
	if start < 0 || end < start {
		return "", false
	}
	return readme[start : end+len(endMarker)], true
}

// replaceBlock swaps the generated block, or appends it when there is none.
func replaceBlock(readme, block string) string {
	if old, ok := extractBlock(readme); ok {
		return strings.Replace(readme, old, block, 1)
	}
	return strings.TrimRight(readme, "\n") + "\n\n" + block + "\n"
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
