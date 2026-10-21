// Command tengrinews serves the Tengrinews news site on :8080.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tengrinews/internal/feed"
	"tengrinews/internal/mongostore"
	"tengrinews/internal/newsdata"
	"tengrinews/internal/web"
	"tengrinews/ui"
)

const (
	listenAddr      = ":8080"
	databaseName    = "news"
	startupTimeout  = 10 * time.Second // MongoDB ping at startup
	shutdownTimeout = 10 * time.Second // in-flight requests on SIGINT/SIGTERM, and the MongoDB disconnect
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(context.Background(), logger, os.Getenv); err != nil {
		logger.Error("tengrinews stopped", "error", err)
		os.Exit(1)
	}
}

// run wires the service and blocks until it is told to stop. No config value is ever logged.
func run(ctx context.Context, logger *slog.Logger, getenv func(string) string) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore default handling: a second Ctrl+C during the drain kills the process at once.
	context.AfterFunc(ctx, stop)

	openCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	store, err := mongostore.Open(openCtx, cfg.mongoURI, databaseName)
	cancel()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := store.Close(closeCtx); err != nil {
			logger.Error("disconnect mongodb", "error", err)
		}
	}()

	nd := newsdata.New(newsdata.DefaultBaseURL, cfg.newsdataKey, newsdataHTTPClient())
	handler, err := web.NewHandler(logger, store, feed.New(nd, logger, time.Now), ui.Files)
	if err != nil {
		return fmt.Errorf("build handler: %w", err)
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	logger.Info("listening", "addr", ln.Addr().String())
	return serve(ctx, newServer(handler, logger), ln, shutdownTimeout)
}

// newsdataHTTPClient bounds every newsdata.io request to 10 s. That is the only bound on a
// feed refresh, which runs detached from the caller's context. Redirects are not followed:
// Go forwards the X-ACCESS-KEY header across hosts, so a 3xx must fail newsdata's 200 check.
func newsdataHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// newServer sets the inbound timeouts. WriteTimeout exceeds the 10 s newsdata.io plus 5 s MongoDB bounds.
func newServer(h http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}

// serve runs srv on ln until it fails or ctx is done. On ctx it stops accepting connections and
// gives in-flight requests timeout to finish; an overrun is an error.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
