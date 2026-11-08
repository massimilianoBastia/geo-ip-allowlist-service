package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/config"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

const testDB = "../../testdata/GeoIP2-Country-Test.mmdb"

// pipeListener is an in-memory net.Listener, so no test opens a socket.
type pipeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

func (l *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *pipeListener) client() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DialContext: l.dial},
		Timeout:   2 * time.Second,
	}
}

// service is a running instance of run.
type service struct {
	client *http.Client
	cancel context.CancelFunc
	result chan error
	logs   *bytes.Buffer
}

func start(t *testing.T, env map[string]string, wrap func(http.Handler) http.Handler) *service {
	t.Helper()
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	s := &service{client: listener.client(), cancel: cancel, result: make(chan error, 1), logs: &bytes.Buffer{}}
	go func() {
		s.result <- run(ctx, func(k string) string { return env[k] }, s.logs, deps{listener: listener, wrap: wrap, version: "dev"})
	}()
	t.Cleanup(cancel)
	return s
}

func (s *service) get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	resp, err := s.client.Get("http://pipe" + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	return resp.StatusCode, m
}

func (s *service) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := s.client.Get("http://pipe/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("/readyz was not 200 within 2s")
}

func TestRunRejectsInvalidConfiguration(t *testing.T) {
	var logs bytes.Buffer
	err := run(context.Background(), func(k string) string {
		return map[string]string{"MAX_REQUEST_BODY_BYTES": "abc"}[k]
	}, &logs, deps{version: "dev"})

	if err == nil || !strings.Contains(err.Error(), "MAX_REQUEST_BODY_BYTES") {
		t.Fatalf("run error = %v, want one naming MAX_REQUEST_BODY_BYTES", err)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"level":"ERROR"`) {
		t.Errorf("logs = %q, want exactly one ERROR line", logs.String())
	}
}

func TestRunServesWithDatabase(t *testing.T) {
	s := start(t, map[string]string{
		"GEOIP_DATABASE_PATH":    testDB,
		"DATABASE_SANITY_CHECKS": "81.2.69.160=GB,2001:218::1=JP",
		"DATABASE_POLL_INTERVAL": "50ms",
	}, nil)
	s.waitReady(t)

	db, err := geoipdb.Open(testDB)
	if err != nil {
		t.Fatal(err)
	}
	wantBuilt := db.Info().BuiltAt.UTC().Format(time.RFC3339)

	status, body := s.get(t, "/version")
	if status != 200 || body["database_built_at"] != wantBuilt || body["version"] != "dev" {
		t.Errorf("/version = %d %v, want 200 with database_built_at %s", status, body, wantBuilt)
	}
	if status, _ := s.get(t, "/livez"); status != 200 {
		t.Errorf("/livez = %d, want 200", status)
	}
}

func TestRunWithoutDatabaseFile(t *testing.T) {
	s := start(t, map[string]string{
		"GEOIP_DATABASE_PATH":    "/does/not/exist.mmdb",
		"DATABASE_POLL_INTERVAL": "50ms",
	}, nil)

	if status, _ := s.get(t, "/readyz"); status != 503 {
		t.Errorf("/readyz = %d, want 503", status)
	}
	if status, _ := s.get(t, "/livez"); status != 200 {
		t.Errorf("/livez = %d, want 200", status)
	}
	if status, body := s.get(t, "/version"); status != 200 || len(body) != 1 || body["version"] != "dev" {
		t.Errorf("/version = %d %v, want 200 {version: dev}", status, body)
	}
}

func TestGracefulShutdownFinishesInFlightRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	wrap := func(next http.Handler) http.Handler {
		mux := http.NewServeMux()
		mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-release
			w.WriteHeader(http.StatusOK)
		})
		mux.Handle("/", next)
		return mux
	}
	s := start(t, map[string]string{"GEOIP_DATABASE_PATH": "/does/not/exist.mmdb", "SHUTDOWN_TIMEOUT": "2s"}, wrap)

	status := make(chan int, 1)
	go func() {
		resp, err := s.client.Get("http://pipe/slow")
		if err != nil {
			status <- -1
			return
		}
		resp.Body.Close()
		status <- resp.StatusCode
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("/slow was not reached")
	}

	s.cancel()
	select {
	case err := <-s.result:
		t.Fatalf("run returned %v while a request was in flight", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	if got := <-status; got != 200 {
		t.Errorf("/slow status = %d, want 200", got)
	}
	select {
	case err := <-s.result:
		if err != nil {
			t.Errorf("run error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return after the request finished")
	}
}

// TestServerTimeouts covers G13: the fixed server timeouts.
func TestServerTimeouts(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer(config.Config{ListenAddr: ":8080"}, http.NotFoundHandler(), nil, logger)
	got := map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	}
	want := map[string]time.Duration{
		"ReadHeaderTimeout": 5 * time.Second,
		"ReadTimeout":       10 * time.Second,
		"WriteTimeout":      10 * time.Second,
		"IdleTimeout":       60 * time.Second,
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %v, want %v", name, got[name], w)
		}
	}
}

// TestCleanStopLogsServerStopped covers G66: a clean stop returns no error and
// logs one INFO line "server stopped".
func TestCleanStopLogsServerStopped(t *testing.T) {
	s := start(t, map[string]string{"GEOIP_DATABASE_PATH": testDB}, nil)
	s.waitReady(t)
	s.cancel()
	if err := <-s.result; err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(s.logs.String()), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		if rec["msg"] == "server stopped" && rec["level"] == "INFO" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("INFO \"server stopped\" lines = %d, want 1; logs:\n%s", n, s.logs)
	}
}

func TestRunReturnsOnListenerFailure(t *testing.T) {
	listener := newPipeListener()
	_ = listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- run(ctx, func(k string) string {
			if k == "GEOIP_DATABASE_PATH" {
				return testDB
			}
			return ""
		}, io.Discard, deps{listener: listener})
	}()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "serve:") {
			t.Fatalf("run error = %v, want serve error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop after listener failure with an active parent context")
	}
}
