package web

import (
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tengrinews/internal/news"
	"tengrinews/ui"
)

// replacement is one intended difference (design D7): old must occur exactly count
// times in the normalized recording and is replaced by new.
type replacement struct {
	old, new string
	count    int
}

const (
	footerInfoRight = `<div class="header-info-right d-flex align-items-center"></div>`
	footerBottom2   = `<div class="header-bottom header-bottom2 "><div class="container-fluid"><div class="row align-items-center"><div class="col-12"></div></div></div></div>`
	copyrightFixed  = `This site was made by Arshat Aitkozha`
)

// The intended differences of design D7, rows 1-9. Nothing else may differ from the recording.
var (
	homeChanges = []replacement{
		{`href="assets/css/`, `href="/assets/css/`, 14}, // 1
		{`this.src='assets/img/tengrinews_image.jpeg'`, `this.src='/assets/img/tengrinews_image.jpeg'`, len(fixtureHomeArticles)}, // 2
		{`class="header-top d-lg-block d-none"`, `class="header-top d-none d-lg-block"`, 1},                                       // 3
		{`class="header-info-left d-flex justify-content-center"`, `class="header-info-left d-flex"`, 1},                          // 4
		{ // 5: the footer gains the post page's header-bottom2 block, after header-top and before the three closing divs
			footerInfoRight + `</div></div></div></div></div></div></div>`,
			footerInfoRight + `</div></div></div></div>` + footerBottom2 + `</div></div></div>`,
			1,
		},
		{`This site was my by Arshat Aitkozha</a>`, copyrightFixed, 1}, // 6
	}
	categoryChanges = []replacement{
		{`This site was by Arshat Aitkozha</a>`, copyrightFixed, 1}, // 7
	}
	postChanges = []replacement{
		{`<title>Magazine News</title>`, `<title>Tengrinews</title>`, 1}, // 8
		{`This site was made by Arshat Aitkozha</a>`, copyrightFixed, 1}, // 9
	}
)

var (
	reComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reSpace   = regexp.MustCompile(`\s+`)
)

// normalize strips HTML comments (html/template drops them), then removes whitespace next
// to tags and collapses the rest (markup moved into partials changes whitespace only).
func normalize(s string) string {
	s = reComment.ReplaceAllString(s, "")
	s = reSpace.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "> ", ">")
	s = strings.ReplaceAll(s, " <", "<")
	return strings.TrimSpace(s)
}

// diffGolden applies changes to the normalized recording and compares the result with the
// normalized rendering. It returns nil only when they are equal and every change matched
// exactly its count.
func diffGolden(recording, rendered string, changes []replacement) error {
	want, got := normalize(recording), normalize(rendered)
	for _, c := range changes {
		if n := strings.Count(want, c.old); n != c.count {
			return fmt.Errorf("change %q occurs %d times in the recording, want %d", c.old, n, c.count)
		}
	}
	for _, c := range changes {
		want = strings.ReplaceAll(want, c.old, c.new)
	}
	if want == got {
		return nil
	}
	i := 0
	for i < len(want) && i < len(got) && want[i] == got[i] {
		i++
	}
	return fmt.Errorf("rendering differs from recording plus intended changes at offset %d\nwant: ...%s\n got: ...%s",
		i, around(want, i), around(got, i))
}

func around(s string, i int) string {
	return s[max(0, i-60):min(len(s), i+60)]
}

type goldenCase struct {
	name    string
	tmpl    *template.Template
	data    any
	changes []replacement
}

func goldenCases(p pages) []goldenCase {
	return []goldenCase{
		{"home", p.home, homeView{
			Categories:  news.Categories(),
			LatestPosts: fixtureHomeArticles,
			HasPrevPage: fixtureHomePage > 1,
			PrevPageNum: fixtureHomePage - 1,
			HasNextPage: fixtureHomePage < fixtureHomeTotal,
			NextPageNum: fixtureHomePage + 1,
		}, homeChanges},
		{"category", p.category, categoryView{
			Categories: news.Categories(),
			Category:   fixtureCategory,
			Articles:   fixtureCategoryArticles,
		}, categoryChanges},
		{"post", p.post, postView{
			Categories: news.Categories(),
			Post:       fixturePost,
		}, postChanges},
	}
}

func renderToString(t *testing.T, tmpl *template.Template, data any) string {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := render(rec, tmpl, data); err != nil {
		t.Fatal(err)
	}
	return rec.Body.String()
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name+".html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGolden(t *testing.T) {
	p, err := parsePages(ui.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range goldenCases(p) {
		t.Run(c.name, func(t *testing.T) {
			if err := diffGolden(readGolden(t, c.name), renderToString(t, c.tmpl, c.data), c.changes); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestGoldenDetectsDrift proves the comparison fails on anything outside the listed changes.
func TestGoldenDetectsDrift(t *testing.T) {
	p, err := parsePages(ui.Files)
	if err != nil {
		t.Fatal(err)
	}
	c := goldenCases(p)[0] // home
	recording, rendered := readGolden(t, c.name), renderToString(t, c.tmpl, c.data)

	tests := []struct {
		name      string
		rendering string
		changes   []replacement
	}{
		{"changed class", strings.Replace(rendered, `class="header-top `, `class="header-topp `, 1), c.changes},
		{"changed text", strings.Replace(rendered, "Previous", "Prev", 1), c.changes},
		{"missing change", rendered, c.changes[1:]},
		{"count too low", rendered, []replacement{{`href="assets/css/`, `href="/assets/css/`, 13}}},
		{"count too high", rendered, []replacement{{`This site was my by Arshat Aitkozha</a>`, copyrightFixed, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := diffGolden(recording, tt.rendering, tt.changes); err == nil {
				t.Error("diffGolden accepted drift")
			}
		})
	}
	if err := diffGolden(recording, rendered, c.changes); err != nil {
		t.Errorf("unmodified rendering rejected: %v", err)
	}
}
