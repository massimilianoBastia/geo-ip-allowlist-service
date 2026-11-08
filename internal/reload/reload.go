// Package reload keeps the current database and replaces it when the file
// changes and the new file passes all checks.
package reload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

// Opener opens the database file at path.
type Opener interface {
	Open(path string) (geoipdb.Database, error)
}

// Statter reports the modification time of the file at path.
type Statter interface {
	ModTime(path string) (time.Time, error)
}

// rejection names the check a new file failed, for the log.
type rejection struct {
	check string
	err   error
}

func (r *rejection) Error() string { return r.check + ": " + r.err.Error() }

// Reloader owns the current database. Check and Run must be called from one
// goroutine; Current is safe from any goroutine.
type Reloader struct {
	path   string
	sanity []geoipdb.SanityCheck
	stat   Statter
	opener Opener
	logger *slog.Logger

	current atomic.Pointer[geoipdb.Database]

	seenMTime   bool
	lastMTime   time.Time
	statFailing bool
}

// New returns a Reloader for the file at path. It logs a warning when sanity
// is empty.
func New(path string, sanity []geoipdb.SanityCheck, stat Statter, opener Opener, logger *slog.Logger) *Reloader {
	if len(sanity) == 0 {
		logger.Warn("sanity lookups are off")
	}
	return &Reloader{path: path, sanity: sanity, stat: stat, opener: opener, logger: logger}
}

// Current returns the loaded database, or nil when none is loaded yet.
func (r *Reloader) Current() geoipdb.Database {
	if db := r.current.Load(); db != nil {
		return *db
	}
	return nil
}

// Run checks the file at once and then on every tick until ctx is done.
func (r *Reloader) Run(ctx context.Context, ticks <-chan time.Time) {
	r.Check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			r.Check()
		}
	}
}

// Check looks at the file once and swaps in a new database if it passes.
// A file whose modification time was already seen is not opened again.
func (r *Reloader) Check() {
	mtime, err := r.stat.ModTime(r.path)
	if err != nil {
		r.logStatFailure(err)
		return
	}
	r.statFailing = false
	if r.seenMTime && mtime.Equal(r.lastMTime) {
		return
	}
	r.seenMTime, r.lastMTime = true, mtime

	db, err := r.load()
	if err != nil {
		var rej *rejection
		if errors.As(err, &rej) {
			r.logger.Error("database rejected", "check", rej.check, "error", rej.err)
		}
		return
	}
	r.current.Store(&db)
	info := db.Info()
	r.logger.Info("database loaded", "database_type", info.Type, "database_built_at", info.BuiltAt)
}

// logStatFailure logs once per failure, not on every tick.
func (r *Reloader) logStatFailure(err error) {
	if r.statFailing {
		return
	}
	r.statFailing = true
	if r.Current() == nil {
		r.logger.Warn("waiting for database file", "error", err)
		return
	}
	r.logger.Error("database file unavailable", "error", err)
}

// load opens the file and runs every check. The error is a *rejection.
func (r *Reloader) load() (geoipdb.Database, error) {
	db, err := r.opener.Open(r.path)
	if err != nil {
		return nil, &rejection{"open", err}
	}
	info := db.Info()
	if err := geoipdb.CheckType(info.Type); err != nil {
		return nil, &rejection{"type", err}
	}
	if cur := r.Current(); cur != nil && !info.BuiltAt.After(cur.Info().BuiltAt) {
		return nil, &rejection{"not_newer", fmt.Errorf("build time %s is not after %s", info.BuiltAt, cur.Info().BuiltAt)}
	}
	if err := r.checkSanity(db); err != nil {
		return nil, &rejection{"sanity", err}
	}
	return db, nil
}

func (r *Reloader) checkSanity(db geoipdb.Database) error {
	for _, c := range r.sanity {
		rec, err := db.Lookup(c.Addr)
		if err != nil {
			return fmt.Errorf("sanity lookup: %w", err)
		}
		if rec.ISOCode != c.ISOCode {
			return fmt.Errorf("sanity lookup gave %q, want %q", rec.ISOCode, c.ISOCode)
		}
	}
	return nil
}

// FileStat is the Statter for the real file system.
type FileStat struct{}

// ModTime returns the modification time of the file at path.
func (FileStat) ModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat database file: %w", err)
	}
	return info.ModTime(), nil
}

// FileOpener is the Opener for the real file system.
type FileOpener struct{}

// Open reads the database file at path.
func (FileOpener) Open(path string) (geoipdb.Database, error) {
	db, err := geoipdb.Open(path)
	if err != nil {
		return nil, err
	}
	return db, nil
}
