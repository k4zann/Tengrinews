package web

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"tengrinews/ui"
)

func TestParsePagesEmbedded(t *testing.T) {
	if _, err := parsePages(ui.Files); err != nil {
		t.Fatal(err)
	}
}

func TestParsePages(t *testing.T) {
	valid := func() fstest.MapFS {
		return fstest.MapFS{
			"templates/layout.html":         {Data: []byte(`{{define "layout"}}{{template "main" .}}{{end}}`)},
			"templates/partials/p.html":     {Data: []byte(`{{define "p"}}x{{end}}`)},
			"templates/pages/home.html":     {Data: []byte(`{{define "main"}}home{{end}}`)},
			"templates/pages/category.html": {Data: []byte(`{{define "main"}}category{{end}}`)},
			"templates/pages/post.html":     {Data: []byte(`{{define "main"}}post{{end}}`)},
		}
	}
	tests := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantErr bool
	}{
		{"valid", func(fstest.MapFS) {}, false},
		{"broken page", func(m fstest.MapFS) {
			m["templates/pages/post.html"] = &fstest.MapFile{Data: []byte(`{{define "main"}}{{.Title{{end}}`)}
		}, true},
		{"broken layout", func(m fstest.MapFS) {
			m["templates/layout.html"] = &fstest.MapFile{Data: []byte(`{{define "layout"}}{{end`)}
		}, true},
		{"missing page", func(m fstest.MapFS) { delete(m, "templates/pages/category.html") }, true},
		{"no partials", func(m fstest.MapFS) { delete(m, "templates/partials/p.html") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.mutate(m)
			_, err := parsePages(m)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePages error = %v, want error: %v", err, tt.wantErr)
			}
		})
	}
}

// spyWriter records whether anything reached the client.
type spyWriter struct {
	header http.Header
	status int // 0 until WriteHeader is called
	body   bytes.Buffer
}

func (s *spyWriter) Header() http.Header         { return s.header }
func (s *spyWriter) WriteHeader(code int)        { s.status = code }
func (s *spyWriter) Write(b []byte) (int, error) { return s.body.Write(b) }

func TestRender(t *testing.T) {
	tmpl := template.Must(template.New("x").Parse(`{{define "layout"}}ok {{.Name}}{{end}}`))

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if err := render(rec, tmpl, struct{ Name string }{"<b>"}); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		if got, want := rec.Body.String(), "ok &lt;b&gt;"; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
	})

	t.Run("failure writes nothing", func(t *testing.T) {
		// Fails after "partial " is already executed: none of it may reach the client.
		failing := template.Must(template.New("x").Parse(`{{define "layout"}}partial {{index .Items 3}}{{end}}`))
		w := &spyWriter{header: http.Header{}}
		if err := render(w, failing, struct{ Items []int }{}); err == nil {
			t.Fatal("render succeeded, want error")
		}
		if w.status != 0 || len(w.header) != 0 || w.body.Len() != 0 {
			t.Errorf("render wrote status=%d header=%v body=%q on failure", w.status, w.header, w.body.String())
		}
	})
}
