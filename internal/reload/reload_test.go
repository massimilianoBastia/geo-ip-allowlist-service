package reload_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/reload"
)

var (
	t0      = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sanity  = []geoipdb.SanityCheck{{Addr: netip.MustParseAddr("81.2.69.160"), ISOCode: "GB"}}
	goodDB  = fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: t0}, iso: "GB"}
	builtAt = func(days int) time.Time { return t0.AddDate(0, 0, days) }
)

type fakeDB struct {
	info geoipdb.Info
	iso  string
}

func (f fakeDB) Info() geoipdb.Info { return f.info }
func (f fakeDB) Lookup(netip.Addr) (geoipdb.Record, error) {
	return geoipdb.Record{Found: true, ISOCode: f.iso, Name: f.iso}, nil
}

// fakeFile is both the Statter and the Opener.
type fakeFile struct {
	mu      sync.Mutex
	mtime   time.Time
	statErr error
	db      geoipdb.Database
	openErr error
	opens   int
}

func (f *fakeFile) ModTime(string) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mtime, f.statErr
}

func (f *fakeFile) Open(string) (geoipdb.Database, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	return f.db, f.openErr
}

func (f *fakeFile) set(mtime time.Time, db geoipdb.Database, openErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mtime, f.db, f.openErr, f.statErr = mtime, db, openErr, nil
}

func (f *fakeFile) openCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opens
}

func newReloader(t *testing.T, file *fakeFile, checks []geoipdb.SanityCheck) (*reload.Reloader, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	return reload.New("db.mmdb", checks, file, file, logger), &buf
}

// logged returns the decoded JSON log lines.
func logged(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func count(recs []map[string]any, level, msg string) int {
	n := 0
	for _, r := range recs {
		if r["level"] == level && r["msg"] == msg {
			n++
		}
	}
	return n
}

func builtOf(r *reload.Reloader) time.Time {
	if db := r.Current(); db != nil {
		return db.Info().BuiltAt
	}
	return time.Time{}
}

func TestMissingFileThenLoad(t *testing.T) {
	file := &fakeFile{statErr: fs.ErrNotExist}
	r, buf := newReloader(t, file, sanity)

	r.Check()
	r.Check()
	if r.Current() != nil {
		t.Fatal("Current is set while the file is missing")
	}
	if n := count(logged(t, buf), "WARN", "waiting for database file"); n != 1 {
		t.Errorf("waiting warnings = %d, want 1", n)
	}

	file.set(t0, goodDB, nil)
	r.Check()
	if got := builtOf(r); !got.Equal(t0) {
		t.Errorf("built at = %v, want %v", got, t0)
	}
	if n := count(logged(t, buf), "INFO", "database loaded"); n != 1 {
		t.Errorf("loaded infos = %d, want 1", n)
	}
}

func TestSameMtimeOpensOnce(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, _ := newReloader(t, file, sanity)

	r.Check()
	r.Check()
	r.Check()
	if got := file.openCount(); got != 1 {
		t.Errorf("opens = %d, want 1", got)
	}
}

func TestRejectionsKeepOldData(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, buf := newReloader(t, file, sanity)
	r.Check()

	tests := []struct {
		name      string
		db        geoipdb.Database
		openErr   error
		wantCheck string
	}{
		{"open fails", nil, errors.New("corrupt"), "open"},
		{"wrong type", fakeDB{info: geoipdb.Info{Type: "GeoLite2-City", BuiltAt: builtAt(10)}, iso: "GB"}, nil, "type"},
		{"not newer", fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(-30)}, iso: "GB"}, nil, "not_newer"},
		{"sanity fails", fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(10)}, iso: "DE"}, nil, "sanity"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mtime := t0.Add(time.Duration(i+1) * time.Hour)
			file.set(mtime, tt.db, tt.openErr)
			opens := file.openCount()
			before := count(logged(t, buf), "ERROR", "database rejected")

			r.Check()
			r.Check() // same rejected mtime: no new open, no new log

			if got := file.openCount() - opens; got != 1 {
				t.Errorf("opens = %d, want 1", got)
			}
			recs := logged(t, buf)
			if got := count(recs, "ERROR", "database rejected") - before; got != 1 {
				t.Fatalf("rejection errors = %d, want 1", got)
			}
			if last := recs[len(recs)-1]; last["check"] != tt.wantCheck {
				t.Errorf("check = %v, want %q", last["check"], tt.wantCheck)
			}
			if got := builtOf(r); !got.Equal(t0) {
				t.Errorf("built at = %v, want the old %v", got, t0)
			}
		})
	}

	file.set(t0.Add(24*time.Hour), fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(5)}, iso: "GB"}, nil)
	r.Check()
	if got := builtOf(r); !got.Equal(builtAt(5)) {
		t.Errorf("after a valid newer file, built at = %v, want %v", got, builtAt(5))
	}
}

func TestFileRemovedKeepsOldData(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, buf := newReloader(t, file, sanity)
	r.Check()

	file.mu.Lock()
	file.statErr = fs.ErrNotExist
	file.mu.Unlock()
	r.Check()
	r.Check()

	if r.Current() == nil {
		t.Error("old data was dropped after the file was removed")
	}
	if n := count(logged(t, buf), "ERROR", "database file unavailable"); n != 1 {
		t.Errorf("removal errors = %d, want 1", n)
	}
}

func TestEmptySanityWarnsOnce(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, buf := newReloader(t, file, nil)
	r.Check()

	wrongType := fakeDB{info: geoipdb.Info{Type: "GeoLite2-City", BuiltAt: builtAt(1)}, iso: "DE"}
	file.set(t0.Add(time.Hour), wrongType, nil)
	r.Check()
	older := fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(-1)}, iso: "DE"}
	file.set(t0.Add(2*time.Hour), older, nil)
	r.Check()
	if got := count(logged(t, buf), "ERROR", "database rejected"); got != 2 {
		t.Errorf("rejections = %d, want 2 (type and not_newer)", got)
	}

	// With no sanity pairs a newer file whose country is "wrong" is accepted.
	file.set(t0.Add(3*time.Hour), fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(2)}, iso: "DE"}, nil)
	r.Check()
	if got := builtOf(r); !got.Equal(builtAt(2)) {
		t.Errorf("built at = %v, want %v", got, builtAt(2))
	}
	if n := count(logged(t, buf), "WARN", "sanity lookups are off"); n != 1 {
		t.Errorf("sanity warnings = %d, want 1", n)
	}
}

func TestRunLoadsAtStartAndStops(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, _ := newReloader(t, file, sanity)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx, make(chan time.Time)) // no tick ever arrives
		close(done)
	}()

	deadline := time.After(time.Second)
	for r.Current() == nil {
		select {
		case <-deadline:
			t.Fatal("database not loaded before the first tick")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop within 1s of cancel")
	}
}

func TestRunChecksOnTick(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, _ := newReloader(t, file, sanity)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx, ticks)

	file.set(t0.Add(time.Hour), fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(3)}, iso: "GB"}, nil)
	sendTick(t, ticks)
	sendTick(t, ticks) // the second send returns only after the first tick was handled
	if got := builtOf(r); !got.Equal(builtAt(3)) {
		t.Errorf("built at = %v, want %v", got, builtAt(3))
	}
}

func sendTick(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("Run did not receive a tick within 1s")
	}
}

func TestSwapDuringLookups(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, _ := newReloader(t, file, sanity)
	r.Check()

	var failures atomic.Int32
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				db := r.Current()
				if db == nil {
					failures.Add(1)
					continue
				}
				if rec, err := db.Lookup(sanity[0].Addr); err != nil || rec.ISOCode != "GB" {
					failures.Add(1)
				}
			}
		}()
	}
	for i := 1; i <= 20; i++ {
		file.set(t0.Add(time.Duration(i)*time.Hour), fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(i)}, iso: "GB"}, nil)
		r.Check()
	}
	close(stop)
	wg.Wait()
	if got := failures.Load(); got != 0 {
		t.Errorf("failed lookups = %d, want 0", got)
	}
	if got := builtOf(r); !got.Equal(builtAt(20)) {
		t.Errorf("built at = %v, want %v", got, builtAt(20))
	}
}

func TestFileAdaptersReadTestDatabase(t *testing.T) {
	const path = "../../testdata/GeoIP2-Country-Test.mmdb"
	if _, err := (reload.FileStat{}).ModTime(path); err != nil {
		t.Errorf("ModTime error = %v", err)
	}
	db, err := (reload.FileOpener{}).Open(path)
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	if got := db.Info().Type; got != "GeoIP2-Country" {
		t.Errorf("Type = %q, want GeoIP2-Country", got)
	}
	if _, err := (reload.FileOpener{}).Open(path + ".missing"); err == nil {
		t.Error("Open of a missing file succeeded")
	}
}

// lookupErrDB fails every lookup.
type lookupErrDB struct{ fakeDB }

func (lookupErrDB) Lookup(netip.Addr) (geoipdb.Record, error) {
	return geoipdb.Record{}, errors.New("lookup failed")
}

// TestSanityLookupErrorRejects covers G65: a sanity pair fails when the lookup
// itself returns an error, and the old data stays.
func TestSanityLookupErrorRejects(t *testing.T) {
	file := &fakeFile{}
	file.set(t0, goodDB, nil)
	r, buf := newReloader(t, file, sanity)
	r.Check()

	bad := lookupErrDB{fakeDB{info: geoipdb.Info{Type: "GeoLite2-Country", BuiltAt: builtAt(10)}, iso: "GB"}}
	file.set(t0.Add(time.Hour), bad, nil)
	r.Check()

	recs := logged(t, buf)
	if got := count(recs, "ERROR", "database rejected"); got != 1 {
		t.Fatalf("rejection errors = %d, want 1", got)
	}
	if last := recs[len(recs)-1]; last["check"] != "sanity" {
		t.Errorf("check = %v, want sanity", last["check"])
	}
	if got := builtOf(r); !got.Equal(t0) {
		t.Errorf("built at = %v, want the old %v", got, t0)
	}
}
