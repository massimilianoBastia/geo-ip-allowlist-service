// Package config reads the service settings from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/iso"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/netclass"
)

// Config holds the validated settings.
type Config struct {
	ListenAddr      string
	DatabasePath    string
	PollInterval    time.Duration
	SanityChecks    []geoipdb.SanityCheck
	MaxBodyBytes    int64
	ProblemBaseURL  string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// Load reads and validates the settings with getenv. An error names the
// variable that is wrong. An empty or unset variable gets its default.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		ListenAddr:     value(getenv, "LISTEN_ADDR", ":8080"),
		DatabasePath:   value(getenv, "GEOIP_DATABASE_PATH", "/usr/share/GeoIP/GeoLite2-Country.mmdb"),
		ProblemBaseURL: value(getenv, "PROBLEM_BASE_URL", "https://geo-service.example/problems/"),
	}
	var err error
	if cfg.PollInterval, err = positiveDuration(getenv, "DATABASE_POLL_INTERVAL", "60s"); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = positiveDuration(getenv, "SHUTDOWN_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}
	if cfg.MaxBodyBytes, err = positiveInt(getenv, "MAX_REQUEST_BODY_BYTES", "16384"); err != nil {
		return Config{}, err
	}
	if cfg.SanityChecks, err = parseSanityChecks(getenv("DATABASE_SANITY_CHECKS")); err != nil {
		return Config{}, fmt.Errorf("DATABASE_SANITY_CHECKS: %w", err)
	}
	if err = cfg.LogLevel.UnmarshalText([]byte(value(getenv, "LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if err = checkBaseURL(cfg.ProblemBaseURL); err != nil {
		return Config{}, fmt.Errorf("PROBLEM_BASE_URL: %w", err)
	}
	cfg.ProblemBaseURL = strings.TrimSuffix(cfg.ProblemBaseURL, "/") + "/"
	return cfg, nil
}

func value(getenv func(string) string, name, fallback string) string {
	if v := getenv(name); v != "" {
		return v
	}
	return fallback
}

func positiveDuration(getenv func(string) string, name, fallback string) (time.Duration, error) {
	d, err := time.ParseDuration(value(getenv, name, fallback))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: must be positive", name)
	}
	return d, nil
}

func positiveInt(getenv func(string) string, name, fallback string) (int64, error) {
	n, err := strconv.ParseInt(value(getenv, name, fallback), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s: must be a positive integer", name)
	}
	return n, nil
}

func checkBaseURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%q is not an absolute URL", s)
	}
	return nil
}

// parseSanityChecks reads "ip=CC,ip=CC". An empty string gives no checks.
func parseSanityChecks(s string) ([]geoipdb.SanityCheck, error) {
	if s == "" {
		return nil, nil
	}
	var checks []geoipdb.SanityCheck
	for _, pair := range strings.Split(s, ",") {
		ip, code, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("entry %q must look like ip=CC", pair)
		}
		addr, err := netclass.Parse(ip)
		if err != nil {
			return nil, fmt.Errorf("entry %q: %w", pair, err)
		}
		if !iso.Valid(code) {
			return nil, fmt.Errorf("entry %q: %q is not an ISO 3166-1 alpha-2 code", pair, code)
		}
		checks = append(checks, geoipdb.SanityCheck{Addr: addr, ISOCode: code})
	}
	return checks, nil
}
