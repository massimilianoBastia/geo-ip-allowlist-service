package config_test

import (
	"log/slog"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/config"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func TestLoadDefaults(t *testing.T) {
	got, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	want := config.Config{
		ListenAddr:      ":8080",
		DatabasePath:    "/usr/share/GeoIP/GeoLite2-Country.mmdb",
		PollInterval:    60 * time.Second,
		MaxBodyBytes:    16384,
		ProblemBaseURL:  "https://geo-service.example/problems/",
		LogLevel:        slog.LevelInfo,
		ShutdownTimeout: 10 * time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadValues(t *testing.T) {
	got, err := config.Load(env(map[string]string{
		"LISTEN_ADDR":            "127.0.0.1:9000",
		"GEOIP_DATABASE_PATH":    "/data/x.mmdb",
		"DATABASE_POLL_INTERVAL": "5s",
		"DATABASE_SANITY_CHECKS": "81.2.69.160=GB,2001:218::1=JP",
		"MAX_REQUEST_BODY_BYTES": "100",
		"PROBLEM_BASE_URL":       "https://example.org/p/",
		"LOG_LEVEL":              "debug",
		"SHUTDOWN_TIMEOUT":       "3s",
	}))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	want := config.Config{
		ListenAddr:     "127.0.0.1:9000",
		DatabasePath:   "/data/x.mmdb",
		PollInterval:   5 * time.Second,
		MaxBodyBytes:   100,
		ProblemBaseURL: "https://example.org/p/",
		LogLevel:       slog.LevelDebug,
		SanityChecks: []geoipdb.SanityCheck{
			{Addr: netip.MustParseAddr("81.2.69.160"), ISOCode: "GB"},
			{Addr: netip.MustParseAddr("2001:218::1"), ISOCode: "JP"},
		},
		ShutdownTimeout: 3 * time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadErrorsNameTheVariable(t *testing.T) {
	tests := []struct{ name, value string }{
		{"MAX_REQUEST_BODY_BYTES", "0"},
		{"MAX_REQUEST_BODY_BYTES", "-1"},
		{"MAX_REQUEST_BODY_BYTES", "abc"},
		{"MAX_REQUEST_BODY_BYTES", "1.5"},
		{"DATABASE_POLL_INTERVAL", "0s"},
		{"DATABASE_POLL_INTERVAL", "abc"},
		{"DATABASE_SANITY_CHECKS", "8.8.8.8"},
		{"DATABASE_SANITY_CHECKS", "x=US"},
		{"DATABASE_SANITY_CHECKS", "8.8.8.8=USA"},
		{"DATABASE_SANITY_CHECKS", "8.8.8.8=XK"},
		{"DATABASE_SANITY_CHECKS", "81.2.69.160=GB,"},
		{"LOG_LEVEL", "verbose"},
		{"PROBLEM_BASE_URL", "/problems/"},
		{"PROBLEM_BASE_URL", "geo-service.example/problems/"},
		{"SHUTDOWN_TIMEOUT", "-1s"},
		{"SHUTDOWN_TIMEOUT", "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"="+tt.value, func(t *testing.T) {
			_, err := config.Load(env(map[string]string{tt.name: tt.value}))
			if err == nil {
				t.Fatal("Load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("error %q does not name %s", err, tt.name)
			}
		})
	}
}

func TestLoadEmptySanityIsNotAnError(t *testing.T) {
	got, err := config.Load(env(map[string]string{"DATABASE_SANITY_CHECKS": ""}))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if len(got.SanityChecks) != 0 {
		t.Errorf("SanityChecks = %v, want none", got.SanityChecks)
	}
}

func TestLoadAddsMissingSlashToProblemBaseURL(t *testing.T) {
	got, err := config.Load(env(map[string]string{"PROBLEM_BASE_URL": "https://example.org/p"}))
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if got.ProblemBaseURL != "https://example.org/p/" {
		t.Errorf("ProblemBaseURL = %q, want %q", got.ProblemBaseURL, "https://example.org/p/")
	}
}
