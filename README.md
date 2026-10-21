# Tengrinews

## Overview
Tengrinews is a web application that displays news articles fetched from an external API. Users can browse articles by category, view details of individual articles.

## Features
- Display news in the main page
- Categorize articles for easy navigation and showing latest news in that category, that fetched from the newdata.io
- View detailed information for each article, except for the news that are showed in the category pages, because newsdata.io api, shows the content, and other necessary values only for paid subscribers

## Technologies Used
- Go (Golang) for backend development, with the standard library `net/http` for routing
- MongoDB for data storage, through the MongoDB Go driver v2
- HTML/CSS/JavaScript for frontend UI
- External API for fetching news data (Newsdata.io)


## Setup
1. Clone the repository: `https://github.com/k4zann/Tengrinews.git`
2. Set the environment variables `NEWSDATA_API_KEY` (your newsdata.io key) and `MONGODB_URI` (your MongoDB connection string)
3. Run the code: `go run ./cmd/tengrinews`

The templates and static assets are embedded in the binary. Build it with `go build ./cmd/tengrinews`, which writes `tengrinews` (`tengrinews.exe` on Windows), then run `./tengrinews` (`.\tengrinews.exe` on Windows) from any directory. It listens on port 8080.

## MongoDB
The app only reads MongoDB: database `news`, collection `articles`, documents shaped `{articles: [...]}`. It never inserts, updates, deletes or creates indexes, so its MongoDB user needs only the `read` role on `news`.

A post page looks the article up by `articles.article_id`. Create this index once, by hand, so the lookup is an index hit instead of a collection scan. Connect as a user that may create indexes (for example one with the `dbAdmin` role on `news`), not as the app's read-only user, and run it in the `news` database (`mongosh "<admin connection string>"`, then `use news`):

```
db.articles.createIndex({"articles.article_id": 1})
```

The store's integration test runs only when `TEST_MONGODB_URI` is set; without it the test is skipped, and CI sets it. It creates and drops throwaway `tengrinews_it_*` databases, so that URI needs write access to them. Point it at a local or CI MongoDB, for example:

```
TEST_MONGODB_URI=mongodb://localhost:27017 go test ./internal/mongostore
```

## Tests and CI
Run the tests with `go test ./...`. The race detector needs cgo and a C compiler, so run `go test -race ./...` only where you have one; CI always does.

The MongoDB integration test is optional locally: it is skipped unless `TEST_MONGODB_URI` is set (see the MongoDB section). To run it, point the variable at a MongoDB you may write throwaway databases to and run `go test -v -run Integration ./internal/mongostore`.

`.github/workflows/ci.yml` runs on every push and pull request, on the latest stable Go, with a `mongo:8.0` service container and `TEST_MONGODB_URI` set, so the integration test always runs there. It checks:
1. `gofmt -l .` lists nothing
2. `go vet ./...` is clean
3. `go test -race ./...` passes, and the integration tests passed instead of being skipped
4. `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` reports no reachable vulnerabilities

## Usage
- Access the web application in your browser at `http://localhost:8080`
- Browse news articles by category, view article details

## Need to Improve
- Search (future work)
- Write my own parser, and schedule it, so that it will parse every morning for the latest news
- Because of using the api, which parses some news not correctly, in the content section the information's written not correctly, with a lot of unnecessary information in it.
- Set the Docker
