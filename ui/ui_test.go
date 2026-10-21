package ui

import (
	"io/fs"
	"testing"
)

func TestFilesEmbedded(t *testing.T) {
	for _, name := range []string{
		"templates/layout.html",
		"templates/partials/header.html",
		"templates/partials/footer.html",
		"templates/pages/home.html",
		"templates/pages/category.html",
		"templates/pages/post.html",
		"assets/css/style.css",
		"assets/scss/_footer.scss", // needs the all: prefix on the embed directive
	} {
		if _, err := fs.Stat(Files, name); err != nil {
			t.Errorf("Files is missing %s: %v", name, err)
		}
	}
}
