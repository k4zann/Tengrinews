package newsdata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"tengrinews/internal/news"
)

const testKey = "k3y-s3cr3t-marker"

func newClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL, testKey, srv.Client())
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestLatestRequestShape(t *testing.T) {
	var got *http.Request
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"status":"success","results":[]}`))
	})

	if _, err := c.Latest(context.Background(), "health"); err != nil {
		t.Fatalf("Latest: %v", err)
	}

	if got.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", got.Method)
	}
	q := got.URL.Query()
	if q.Get("category") != "health" || q.Get("language") != "en,ru" || len(q) != 2 {
		t.Errorf("decoded query = %v, want only category=health and language=en,ru", q)
	}
	if h := got.Header.Get("X-ACCESS-KEY"); h != testKey {
		t.Errorf("X-ACCESS-KEY = %q, want the key", h)
	}
	raw := got.URL.String()
	if strings.Contains(strings.ToLower(raw), "apikey") || strings.Contains(raw, testKey) {
		t.Errorf("request URL %q carries the key", raw)
	}
}

func TestLatestSuccess(t *testing.T) {
	body := `{"status":"success","totalResults":3,"nextPage":null,"results":[
		{"article_id":"a1","title":"First","link":"https://x.test/1","image_url":"https://img.test/1.jpg","category":["business","top"]},
		{"article_id":"a2","title":"Second","link":"https://x.test/2","image_url":null,"category":null},
		{"article_id":"a3","title":"Third"}
	]}`
	c := newClient(t, reply(http.StatusOK, body))

	got, err := c.Latest(context.Background(), "business")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	want := []news.Article{
		{Title: "First", ImageURL: "https://img.test/1.jpg", Category: []string{"business", "top"}},
		{Title: "Second"},
		{Title: "Third"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("articles = %+v, want %+v", got, want)
	}
}

func TestLatestEmptyResults(t *testing.T) {
	c := newClient(t, reply(http.StatusOK, `{"status":"success","results":[]}`))

	got, err := c.Latest(context.Background(), "food")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d articles, want 0", len(got))
	}
}

func TestLatestFailures(t *testing.T) {
	// Every body echoes the key and a message so the test can prove neither reaches the error.
	errBody := func(code string) string {
		return `{"status":"error","results":{"message":"leaky-message ` + testKey + `","code":"` + code + `"}}`
	}
	tests := []struct {
		name   string
		status int
		body   string
		want   []string
	}{
		{"401", http.StatusUnauthorized, errBody("Unauthorized"), []string{"status 401", `"Unauthorized"`}},
		{"429", http.StatusTooManyRequests, errBody("RateLimitExceeded"), []string{"status 429", `"RateLimitExceeded"`}},
		{"500 with a text body", http.StatusInternalServerError, "boom leaky-message " + testKey, []string{"status 500"}},
		{"200 with invalid JSON", http.StatusOK, "<html>leaky-message " + testKey + "</html>", []string{"decode response"}},
		{"200 with status error", http.StatusOK, errBody("Unauthorized"), []string{`upstream status "error"`, `"Unauthorized"`}},
		{"200 without results", http.StatusOK, `{"status":"success"}`, []string{"decode results"}},
		{"200 body over 4 MiB", http.StatusOK, `{"status":"success","results":[]` + strings.Repeat(" ", maxBody) + `}`, []string{"decode response"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, reply(tt.status, tt.body))

			got, err := c.Latest(context.Background(), "science")
			if err == nil {
				t.Fatalf("Latest = %+v, want an error", got)
			}
			msg := err.Error()
			for _, w := range append(tt.want, "science") {
				if !strings.Contains(msg, w) {
					t.Errorf("error %q does not contain %q", msg, w)
				}
			}
			for _, leak := range []string{testKey, "leaky-message"} {
				if strings.Contains(msg, leak) {
					t.Errorf("error %q leaks %q", msg, leak)
				}
			}
		})
	}
}

func TestLatestDeadlineExceeded(t *testing.T) {
	release := make(chan struct{})
	c := newClient(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) }) // runs before the server closes, so the handler never hangs it

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Latest(ctx, "sports")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error %q leaks the key", err)
	}
}

func TestLatestTransportErrorHasNoKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := New(srv.URL, testKey, srv.Client())
	srv.Close() // connection refused from here on

	_, err := c.Latest(context.Background(), "sports")
	if err == nil {
		t.Fatal("Latest succeeded against a closed server")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error %q leaks the key", err)
	}
}
