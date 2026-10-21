package web

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusWriter remembers the status code so the middleware can log it and tell
// whether a response was already started.
type statusWriter struct {
	http.ResponseWriter
	status int // 0 until a header is written
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController (and so Flush, deadlines, ...) reach the real writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logRequests writes one Info record per request: method, path (without the query), status, duration.
// The record is deferred, so a request that ends in a panic is still logged.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		defer func() {
			status := sw.status
			if status == 0 {
				status = http.StatusOK // net/http's implicit status when the handler wrote nothing
			}
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", status, "duration", time.Since(start))
		}()
		next.ServeHTTP(sw, r)
	})
}

// recoverPanics turns a handler panic into a logged 500, if the response has not started.
// If it has, the client already holds a partial page that would look complete, so the
// connection is aborted instead. http.ErrAbortHandler is net/http's way to abort a
// response, so it is passed on as is.
func recoverPanics(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			log.Error("panic serving request", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			if sw.status != 0 {
				panic(http.ErrAbortHandler)
			}
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()
		next.ServeHTTP(sw, r)
	})
}
