package mongostore_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"tengrinews/internal/mongostore"
)

const secret = "s3cr3t"

func TestOpenBadURIHidesSecret(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"driver echoes option value", "mongodb://user:" + secret + "@localhost/?readPreference=bogus-" + secret},
		{"bad port", "mongodb://user:" + secret + "@localhost:notaport/"},
		{"no scheme", secret},
		{"invalid combination", "mongodb://user:" + secret + "@a,b/?directConnection=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := mongostore.Open(context.Background(), tt.uri, "news")
			if err == nil {
				_ = s.Close(context.Background())
				t.Fatal("Open succeeded, want error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error leaks the secret: %v", err)
			}
			if err.Error() != "invalid MONGODB_URI or SRV lookup failed" {
				t.Errorf("error = %q, want the fixed text", err)
			}
		})
	}
}

func TestOpenUnreachableFailsFastHidingSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := mongostore.Open(ctx, "mongodb://user:"+secret+"@127.0.0.1:1/?serverSelectionTimeoutMS=300", "news")
	if err == nil {
		_ = s.Close(context.Background())
		t.Fatal("Open succeeded, want error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks the secret: %v", err)
	}
	// The 300 ms server selection timeout must end the ping, not the 5 s context.
	if ctx.Err() != nil {
		t.Errorf("Open ran until the context expired: %v", err)
	}
}
