// Command geoip-service answers whether an IP address is in an allowed country.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/config"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/httpapi"
	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/reload"
)

// version is set at build time with -ldflags "-X main.version=<value>".
var version = "dev"

// deps holds what tests replace. A nil listener means listen on LISTEN_ADDR,
// a nil wrap means no extra handler layer.
type deps struct {
	listener net.Listener
	wrap     func(http.Handler) http.Handler
	version  string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	// run has already logged the error.
	if err := run(ctx, os.Getenv, os.Stdout, deps{version: version}); err != nil {
		stop()
		os.Exit(1)
	}
}

// run builds the service and serves until ctx is done. It logs an error before
// returning it.
func run(ctx context.Context, getenv func(string) string, stdout io.Writer, d deps) error {
	logger := newLogger(stdout, slog.LevelInfo)
	cfg, err := config.Load(getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return fmt.Errorf("load configuration: %w", err)
	}
	logger = newLogger(stdout, cfg.LogLevel)

	listener, err := listen(d.listener, cfg.ListenAddr)
	if err != nil {
		logger.Error("cannot listen", "error", err)
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	reloader := reload.New(cfg.DatabasePath, cfg.SanityChecks, reload.FileStat{}, reload.FileOpener{}, logger)
	var background sync.WaitGroup
	defer func() {
		cancel()
		background.Wait()
	}()
	background.Add(1)
	go func() {
		defer background.Done()
		poll(ctx, reloader, cfg.PollInterval)
	}()

	srv := newServer(cfg, httpapi.New(reloader, httpapi.Settings{
		MaxBodyBytes:   cfg.MaxBodyBytes,
		ProblemBaseURL: cfg.ProblemBaseURL,
		Version:        d.version,
	}, logger), d.wrap, logger)
	if err := serve(ctx, srv, listener, cfg.ShutdownTimeout); err != nil {
		logger.Error("server stopped", "error", err)
		return err
	}
	logger.Info("server stopped")
	return nil
}

func newLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

func listen(l net.Listener, addr string) (net.Listener, error) {
	if l != nil {
		return l, nil
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %q: %w", addr, err)
	}
	return l, nil
}

// poll checks the database file at start and then every interval until ctx is done.
func poll(ctx context.Context, r *reload.Reloader, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	r.Run(ctx, ticker.C)
}

func newServer(cfg config.Config, handler http.Handler, wrap func(http.Handler) http.Handler, logger *slog.Logger) *http.Server {
	if wrap != nil {
		handler = wrap(handler)
	}
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

// serve runs srv until ctx is done, then waits up to timeout for requests in
// flight to finish.
func serve(ctx context.Context, srv *http.Server, l net.Listener, timeout time.Duration) error {
	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(l) }()

	select {
	case err := <-failed:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	if err := <-failed; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
