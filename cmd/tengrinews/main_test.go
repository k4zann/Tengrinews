package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"tengrinews/internal/newsdata"
)

const secret = "s3cr3t"

func TestRunMissingEnv(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	err := run(context.Background(), logger, envOf(map[string]string{"MONGODB_URI": "mongodb://user:" + secret + "@127.0.0.1:1/"}))
	if err == nil || !strings.Contains(err.Error(), "NEWSDATA_API_KEY") {
		t.Fatalf("run error = %v, want one naming NEWSDATA_API_KEY", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(logs.String(), secret) {
		t.Errorf("secret leaked: error %q, logs %q", err, logs.String())
	}
	if strings.Contains(logs.String(), "listening") {
		t.Errorf("service started listening: %s", logs.String())
	}
}

func TestRunUnreachableMongoHidesSecret(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uri := "mongodb://user:" + secret + "@127.0.0.1:1/?serverSelectionTimeoutMS=300"
	err := run(ctx, logger, envOf(map[string]string{"NEWSDATA_API_KEY": "key", "MONGODB_URI": uri}))
	if err == nil {
		t.Fatal("run succeeded against an unreachable MongoDB")
	}
	if ctx.Err() != nil {
		t.Errorf("run ran until the test deadline: %v", err)
	}
	// main logs run's error, so it is part of the captured output too.
	logger.Error("tengrinews stopped", "error", err)
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), uri) {
		t.Errorf("logs leak the credentials: %s", logs.String())
	}
	if strings.Contains(logs.String(), "listening") {
		t.Errorf("service started listening: %s", logs.String())
	}
}

// startServe runs serve on 127.0.0.1:0 and returns the base URL and a channel carrying serve's result.
func startServe(ctx context.Context, t *testing.T, h http.Handler, timeout time.Duration) (string, *http.Server, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(h, slog.New(slog.DiscardHandler))
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, timeout) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String(), srv, done
}

func TestServeFinishesInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		finished.Store(true)
		_, _ = io.WriteString(w, "done")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base, _, done := startServe(ctx, t, h, 10*time.Second)

	type result struct {
		status int
		body   string
		err    error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get(base)
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		got <- result{status: resp.StatusCode, body: string(b), err: err}
	}()

	<-started
	cancel()
	// Shutdown closes the listener first: wait until new connections are refused.
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
		if err != nil {
			break
		}
		_ = c.Close()
		if time.Now().After(deadline) {
			t.Fatal("new connections still accepted after shutdown began")
		}
	}
	close(release)

	if err := <-done; err != nil {
		t.Fatalf("serve = %v, want nil", err)
	}
	if !finished.Load() {
		t.Error("serve returned before the in-flight handler finished")
	}
	if r := <-got; r.err != nil || r.status != http.StatusOK || r.body != "done" {
		t.Errorf("in-flight request = %+v, want 200 done", r)
	}
}

func TestServeShutdownDeadline(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base, srv, done := startServe(ctx, t, h, 50*time.Millisecond)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		if resp, err := http.Get(base); err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("serve = %v, want an error wrapping context.DeadlineExceeded", err)
	}
	close(release)
	_ = srv.Close()
	<-clientDone
}

func TestNewServerTimeouts(t *testing.T) {
	srv := newServer(http.NotFoundHandler(), slog.New(slog.DiscardHandler))
	for _, tt := range []struct {
		name      string
		got, want time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout, 5 * time.Second},
		{"ReadTimeout", srv.ReadTimeout, 10 * time.Second},
		{"WriteTimeout", srv.WriteTimeout, 20 * time.Second},
		{"IdleTimeout", srv.IdleTimeout, 60 * time.Second},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

// pipeListener hands out the connections sent on conns, then blocks in Accept until closed.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *pipeListener) Addr() net.Addr { return &net.TCPAddr{} }

// A client that sends only part of its request headers is cut off after 5 s. The synctest
// clock is fake, so the 5 s cost no real time and the elapsed time is exact.
func TestServerClosesSlowHeaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, client := net.Pipe()
		ln := &pipeListener{conns: make(chan net.Conn, 1), closed: make(chan struct{})}
		ln.conns <- server
		srv := newServer(http.NotFoundHandler(), slog.New(slog.DiscardHandler))
		go func() { _ = srv.Serve(ln) }()
		defer srv.Close()
		defer client.Close()

		start := time.Now()
		if _, err := io.WriteString(client, "GET / HTTP/1.1\r\nHost: example.test\r\n"); err != nil { // no blank line: headers never complete
			t.Fatal(err)
		}
		b, err := io.ReadAll(client) // returns when the server closes the connection
		if err != nil || len(b) != 0 {
			t.Errorf("read %q, %v; want the connection closed without a response", b, err)
		}
		if elapsed := time.Since(start); elapsed < 5*time.Second || elapsed >= 6*time.Second {
			t.Errorf("connection closed after %v, want 5s", elapsed)
		}
	})
}

func TestNewsdataHTTPClientTimeout(t *testing.T) {
	if got := newsdataHTTPClient().Timeout; got != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s: it is the only bound on a feed refresh", got)
	}
}

func TestNewsdataHTTPClientDoesNotFollowRedirects(t *testing.T) {
	var reached atomic.Int32 // requests that arrived at the redirect target, with or without the key
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		_, _ = io.WriteString(w, `{"status":"success","results":[]}`)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	const key = "k3y-marker"
	_, err := newsdata.New(redirector.URL, key, newsdataHTTPClient()).Latest(context.Background(), "business")
	if err == nil {
		t.Fatal("Latest followed the redirect and succeeded, want an error")
	}
	if n := reached.Load(); n != 0 {
		t.Errorf("%d request(s) reached the redirect target, want 0: the key header would have gone with them", n)
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("error leaks the key: %v", err)
	}
}
