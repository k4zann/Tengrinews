// Package newsdata is a minimal client for the newsdata.io latest-news API.
package newsdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"tengrinews/internal/news"
)

// DefaultBaseURL is the production newsdata.io endpoint.
const DefaultBaseURL = "https://newsdata.io/api/1/news"

const maxBody = 4 << 20 // 4 MiB

// Client calls newsdata.io. The API key travels only in the X-ACCESS-KEY header.
type Client struct {
	baseURL string
	apiKey  string
	hc      *http.Client
}

// New returns a Client. The caller sets the timeout on hc.
func New(baseURL, apiKey string, hc *http.Client) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, hc: hc}
}

type envelope struct {
	Status  string          `json:"status"`
	Results json.RawMessage `json:"results"`
}

// wireArticle is the part of a newsdata.io result the category page renders.
type wireArticle struct {
	Title    string   `json:"title"`
	ImageURL string   `json:"image_url"`
	Category []string `json:"category"`
}

// Latest returns the latest articles of a category. Errors name the category,
// the HTTP status and the upstream error code, never the key or the body.
func (c *Client) Latest(ctx context.Context, category string) ([]news.Article, error) {
	q := url.Values{"category": {category}, "language": {"en,ru"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("newsdata %s: build request: %w", category, err)
	}
	req.Header.Set("X-ACCESS-KEY", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("newsdata %s: request: %w", category, err)
	}
	defer resp.Body.Close()

	var env envelope
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&env)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("newsdata %s: status %d%s", category, resp.StatusCode, upstreamCode(env.Results))
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("newsdata %s: decode response: %w", category, decodeErr)
	}
	if env.Status != "success" {
		return nil, fmt.Errorf("newsdata %s: upstream status %q%s", category, env.Status, upstreamCode(env.Results))
	}

	var wire []wireArticle
	if err := json.Unmarshal(env.Results, &wire); err != nil {
		return nil, fmt.Errorf("newsdata %s: decode results: %w", category, err)
	}
	articles := make([]news.Article, len(wire))
	for i, w := range wire {
		articles[i] = news.Article{Title: w.Title, ImageURL: w.ImageURL, Category: w.Category}
	}
	return articles, nil
}

// upstreamCode returns ", code <results.code>" for an error body, or "".
func upstreamCode(results json.RawMessage) string {
	var e struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(results, &e) != nil || e.Code == "" {
		return ""
	}
	return fmt.Sprintf(", code %q", e.Code)
}
