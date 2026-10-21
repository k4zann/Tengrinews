package mongostore_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"tengrinews/internal/mongostore"
	"tengrinews/internal/news"
)

// openThrowaway returns a store on a fresh database that is dropped when the test ends,
// plus the raw collection to seed it. It skips when TEST_MONGODB_URI is unset, except
// under CI, where an unset URI is a failure so a broken workflow can't skip silently.
func openThrowaway(t *testing.T) (*mongostore.Store, *mongo.Collection) {
	t.Helper()
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_MONGODB_URI must be set when CI is set")
		}
		t.Skip("TEST_MONGODB_URI not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	raw, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	dbName := fmt.Sprintf("tengrinews_it_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		if err := raw.Database(dbName).Drop(cctx); err != nil {
			t.Errorf("drop %s: %v", dbName, err)
		}
		_ = raw.Disconnect(cctx)
	})

	store, err := mongostore.Open(ctx, uri, dbName)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		_ = store.Close(cctx)
	})
	return store, raw.Database(dbName).Collection("articles")
}

// seedDoc is one stored article, including the `link` the store must ignore.
func seedDoc(id string, category []string) bson.M {
	return bson.M{
		"article_id":  id,
		"title":       "title " + id,
		"description": "description " + id,
		"content":     "content " + id,
		"pubDate":     "2026-10-22 10:00:00",
		"image_url":   "https://example.com/" + id + ".jpg",
		"category":    category,
		"creator":     []string{"creator " + id},
		"link":        "https://example.com/source/" + id,
	}
}

func TestIntegrationReadsSeededCollection(t *testing.T) {
	store, coll := openThrowaway(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Four documents: 7 articles, one without an `articles` field, one with an empty array, and
	// 5 articles. The two empty ones sit between the others and must change neither count nor
	// pages. a4 has an empty category.
	var first, second bson.A
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("a%d", i)
		category := []string{"business"}
		if id == "a4" {
			category = []string{}
		}
		if i <= 7 {
			first = append(first, seedDoc(id, category))
		} else {
			second = append(second, seedDoc(id, category))
		}
	}
	for i, doc := range []bson.M{{"articles": first}, {}, {"articles": bson.A{}}, {"articles": second}} {
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			t.Fatalf("seed document %d: %v", i, err)
		}
	}

	t.Run("count", func(t *testing.T) {
		n, err := store.CountArticles(ctx)
		if err != nil || n != 12 {
			t.Fatalf("CountArticles = %d, %v; want 12, nil", n, err)
		}
	})

	t.Run("list pages in storage order", func(t *testing.T) {
		page1, err := store.ListArticles(ctx, 0, 10)
		if err != nil {
			t.Fatalf("ListArticles(0, 10): %v", err)
		}
		if len(page1) != 10 {
			t.Fatalf("ListArticles(0, 10) returned %d articles, want 10", len(page1))
		}
		for i, a := range page1 {
			if want := fmt.Sprintf("a%d", i+1); a.ID != want {
				t.Errorf("page1[%d].ID = %q, want %q", i, a.ID, want)
			}
		}
		// The list projection carries only what the home page renders.
		if a := page1[0]; a.Title != "title a1" || a.ImageURL != "https://example.com/a1.jpg" ||
			!reflect.DeepEqual(a.Category, []string{"business"}) || a.Description != "" || a.Content != "" {
			t.Errorf("page1[0] = %+v, want only id, title, category and image_url", a)
		}
		if len(page1[3].Category) != 0 {
			t.Errorf("page1[3].Category = %v, want empty", page1[3].Category)
		}

		page2, err := store.ListArticles(ctx, 10, 10)
		if err != nil {
			t.Fatalf("ListArticles(10, 10): %v", err)
		}
		if len(page2) != 2 || page2[0].ID != "a11" || page2[1].ID != "a12" {
			t.Errorf("ListArticles(10, 10) = %+v, want a11 and a12", page2)
		}
	})

	t.Run("by id returns every field", func(t *testing.T) {
		// a9 lives in the second document: the positional projection must pick it alone.
		got, err := store.ArticleByID(ctx, "a9")
		if err != nil {
			t.Fatalf("ArticleByID(a9): %v", err)
		}
		want := news.Article{
			ID:          "a9",
			Title:       "title a9",
			Description: "description a9",
			Content:     "content a9",
			ImageURL:    "https://example.com/a9.jpg",
			PubDate:     "2026-10-22 10:00:00",
			Category:    []string{"business"},
			Creator:     []string{"creator a9"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ArticleByID(a9) = %+v, want %+v", got, want)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		if _, err := store.ArticleByID(ctx, "nope"); !errors.Is(err, news.ErrNotFound) {
			t.Errorf("ArticleByID(nope) error = %v, want news.ErrNotFound", err)
		}
	})
}

func TestIntegrationCountEmptyCollection(t *testing.T) {
	store, _ := openThrowaway(t)
	n, err := store.CountArticles(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("CountArticles on an empty collection = %d, %v; want 0, nil", n, err)
	}
}
