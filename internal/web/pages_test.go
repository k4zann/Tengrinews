package web

import (
	"strings"
	"testing"

	"tengrinews/internal/news"
	"tengrinews/ui"
)

func mustPages(t *testing.T) pages {
	t.Helper()
	p, err := parsePages(ui.Files)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// renderAll renders the three pages with the given article in each page's article slot
// and category as the category page's breadcrumb (the value from the request).
func renderAll(t *testing.T, a news.Article, category string) map[string]string {
	t.Helper()
	p := mustPages(t)
	cats := news.Categories()
	return map[string]string{
		"home":     renderToString(t, p.home, homeView{Categories: cats, LatestPosts: []news.Article{a}}),
		"category": renderToString(t, p.category, categoryView{Categories: cats, Category: category, Articles: []news.Article{a}}),
		"post":     renderToString(t, p.post, postView{Categories: cats, Post: a}),
	}
}

// section returns the first <tag>...</tag> of s, tags included.
func section(t *testing.T, s, tag string) string {
	t.Helper()
	start := strings.Index(s, "<"+tag+">")
	end := strings.Index(s, "</"+tag+">")
	if start < 0 || end < start {
		t.Fatalf("no <%s> section", tag)
	}
	return s[start : end+len("</"+tag+">")]
}

func TestSharedFrame(t *testing.T) {
	out := renderAll(t, fixturePost, fixtureCategory)
	for _, tag := range []string{"head", "header", "footer"} {
		want := section(t, out["home"], tag)
		for _, page := range []string{"category", "post"} {
			if got := section(t, out[page], tag); got != want {
				t.Errorf("<%s> of %s page differs from home page", tag, page)
			}
		}
	}
	for page, html := range out {
		if !strings.Contains(html, "<title>Tengrinews</title>") {
			t.Errorf("%s page: title is not Tengrinews", page)
		}
		if !strings.Contains(normalize(html), "Copyright &copy;<script>document.write(new Date().getFullYear());</script>All rights reserved | This site was made by Arshat Aitkozha</p>") {
			t.Errorf("%s page: copyright line is not the fixed wording without a stray </a>", page)
		}
		for _, c := range news.Categories() {
			if link := `<li><a href="/category/` + c.Slug + `">` + c.Name + `</a></li>`; !strings.Contains(html, link) {
				t.Errorf("%s page: nav lacks %s", page, link)
			}
		}
		for _, rel := range []string{`href="assets/`, `src="assets/`, `src='assets/`, `href='assets/`} {
			if strings.Contains(html, rel) {
				t.Errorf("%s page has relative asset URL %s", page, rel)
			}
		}
	}
}

func TestEscaping(t *testing.T) {
	const script = "<script>alert(1)</script>"
	hostile := news.Article{
		ID:          "id1",
		Title:       script,
		Description: "<b>bold</b>",
		Content:     "<img src=x onerror=alert(1)>",
		ImageURL:    "javascript:alert(1)",
		Category:    []string{script},
	}
	out := renderAll(t, hostile, script)

	// The category label and the breadcrumb, which render Category[0] and the requested category.
	const escaped = "&lt;script&gt;alert(1)&lt;/script&gt;"
	for page, want := range map[string][]string{
		"home":     {"<p>" + escaped + "</p>"},
		"category": {"<p>" + escaped + "</p>", `<li class="breadcrumb-item"><a href="#">` + escaped + `</a></li>`},
		"post":     {`<li class="breadcrumb-item"><a href="#">` + escaped + `</a></li>`},
	} {
		for _, w := range want {
			if !strings.Contains(out[page], w) {
				t.Errorf("%s page: category value is not escaped, want %s", page, w)
			}
		}
	}

	for page, html := range out {
		if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("%s page: title is not escaped", page)
		}
		if strings.Contains(html, "<script>alert(1)") {
			t.Errorf("%s page: title became a script element", page)
		}
		if strings.Contains(html, "javascript:") {
			t.Errorf("%s page: unsafe image URL survived", page)
		}
	}
	post := out["post"]
	if !strings.Contains(post, "&lt;img src=x onerror=alert(1)&gt;") || strings.Contains(post, "<img src=x") {
		t.Error("post page: content is not rendered as text")
	}
	if !strings.Contains(post, "&lt;b&gt;bold&lt;/b&gt;") {
		t.Error("post page: description is not escaped")
	}
}

func TestMissingOptionalFields(t *testing.T) {
	bare := news.Article{ID: "id1", Title: "Bare"} // no category, creator or image
	out := renderAll(t, bare, fixtureCategory)

	if !strings.Contains(out["home"], "<p></p>") {
		t.Error("home page: empty category list does not give an empty label")
	}
	if !strings.Contains(out["category"], "<p></p>") {
		t.Error("category page: empty category list does not give an empty label")
	}
	if !strings.Contains(out["post"], `<li class="breadcrumb-item"><a href="#"></a></li>`) {
		t.Error("post page: empty category list does not give an empty breadcrumb item")
	}
	if strings.Contains(out["post"], "Creator") {
		t.Error("post page: Creator line shown without creators")
	}
	for page, html := range out {
		if !strings.Contains(html, `src=""`) {
			t.Errorf("%s page: empty image URL not rendered as empty src", page)
		}
	}

	if !strings.Contains(renderAll(t, fixturePost, fixtureCategory)["post"], "<strong>Creator:</strong> Aida Nurlan") {
		t.Error("post page: Creator line missing with a creator")
	}
}
