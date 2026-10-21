package main

import (
	"os"
	"strings"
	"testing"
)

// The read-only service never creates indexes, so the README must give operators the exact command.
func TestReadmeDocumentsIndexCommand(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	const cmd = `db.articles.createIndex({"articles.article_id": 1})`
	if !strings.Contains(string(b), cmd) {
		t.Errorf("README.md does not contain %s", cmd)
	}
}
