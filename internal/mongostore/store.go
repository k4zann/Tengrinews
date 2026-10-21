// Package mongostore reads articles from MongoDB. It is read-only: it never
// inserts, updates, deletes or creates indexes.
package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"tengrinews/internal/news"
)

const (
	collectionName = "articles"
	// opTimeout is the client Timeout. v2 replaces it with any ctx deadline, so it applies only
	// when the caller's ctx has no deadline. It also bounds the disconnect after a failed ping.
	opTimeout = 5 * time.Second
)

// errInvalidURI is fixed text: the driver's own parse errors may echo parts of the URI.
var errInvalidURI = errors.New("invalid MONGODB_URI or SRV lookup failed")

// Store reads the articles collection. It is safe for concurrent use.
type Store struct {
	client *mongo.Client
	coll   *mongo.Collection
}

// articleDoc is one element of the `articles` array. Missing fields decode to zero values.
// The stored `link` field is deliberately not mapped.
type articleDoc struct {
	ID          string   `bson:"article_id"`
	Title       string   `bson:"title"`
	Description string   `bson:"description"`
	Content     string   `bson:"content"`
	ImageURL    string   `bson:"image_url"`
	PubDate     string   `bson:"pubDate"`
	Category    []string `bson:"category"`
	Creator     []string `bson:"creator"`
}

func (d articleDoc) article() news.Article {
	return news.Article{
		ID:          d.ID,
		Title:       d.Title,
		Description: d.Description,
		Content:     d.Content,
		ImageURL:    d.ImageURL,
		PubDate:     d.PubDate,
		Category:    d.Category,
		Creator:     d.Creator,
	}
}

// Open connects to uri and pings it; ctx bounds the ping. Operations whose ctx has no deadline
// are bounded to 5 s by the client Timeout. Errors never contain the URI.
func Open(ctx context.Context, uri, database string) (*Store, error) {
	// Connect contacts no server, so an error here is a bad URI or option. For mongodb+srv://
	// the URI parse also does SRV/TXT DNS lookups, so a DNS failure is reported the same way.
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(opTimeout))
	if err != nil {
		return nil, errInvalidURI
	}
	if err := client.Ping(ctx, nil); err != nil {
		// ctx may be the reason the ping failed, so disconnect on a fresh one.
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opTimeout)
		defer cancel()
		_ = client.Disconnect(dctx)
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}
	return &Store{client: client, coll: client.Database(database).Collection(collectionName)}, nil
}

// Close disconnects from MongoDB.
func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// CountArticles returns the number of articles across all documents.
func (s *Store) CountArticles(ctx context.Context) (int, error) {
	pipeline := bson.A{bson.M{"$group": bson.M{
		"_id": nil,
		"n": bson.M{"$sum": bson.M{"$cond": bson.A{
			bson.M{"$isArray": bson.A{"$articles"}},
			bson.M{"$size": "$articles"},
			0,
		}}},
	}}}
	cur, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("count articles: %w", err)
	}
	var out []struct {
		N int `bson:"n"`
	}
	if err := cur.All(ctx, &out); err != nil {
		return 0, fmt.Errorf("count articles: %w", err)
	}
	if len(out) == 0 { // no documents
		return 0, nil
	}
	return out[0].N, nil
}

// ListArticles returns articles skip..skip+limit in storage order, with only the
// fields the home page renders (ID, Title, Category, ImageURL).
func (s *Store) ListArticles(ctx context.Context, skip, limit int) ([]news.Article, error) {
	pipeline := bson.A{
		bson.M{"$unwind": "$articles"},
		bson.M{"$replaceRoot": bson.M{"newRoot": "$articles"}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
		bson.M{"$project": bson.M{"article_id": 1, "title": 1, "category": 1, "image_url": 1}},
	}
	cur, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	var docs []articleDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	out := make([]news.Article, len(docs))
	for i, d := range docs {
		out[i] = d.article()
	}
	return out, nil
}

// ArticleByID returns the article with that article_id, or news.ErrNotFound.
// One query returns only the matching array element.
func (s *Store) ArticleByID(ctx context.Context, id string) (news.Article, error) {
	var doc struct {
		Articles []articleDoc `bson:"articles"`
	}
	err := s.coll.FindOne(ctx,
		bson.M{"articles.article_id": id},
		options.FindOne().SetProjection(bson.M{"articles.$": 1}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return news.Article{}, news.ErrNotFound
	}
	if err != nil {
		return news.Article{}, fmt.Errorf("find article: %w", err)
	}
	if len(doc.Articles) == 0 {
		return news.Article{}, news.ErrNotFound
	}
	return doc.Articles[0].article(), nil
}
