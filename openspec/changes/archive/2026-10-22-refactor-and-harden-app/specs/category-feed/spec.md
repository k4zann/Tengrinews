# Spec Delta

## Purpose

Defines how the service gets the latest news for a category from newsdata.io: which categories exist, how the API key is sent and protected, and how results are cached so pages stay up within the free-tier quota and during upstream outages.

## ADDED Requirements

### Requirement: Allowed categories
The service SHALL know exactly nine categories, as slug and display name: business/Business, education/Education, environment/Environment, food/Food, health/Health, lifestyle/Lifestyle, science/Science, sports/Sports, technology/Technology. A requested value SHALL be lowercased, then matched exactly against these slugs. A value whose lowercase form matches no slug SHALL never be sent to newsdata.io nor become a cache entry.

#### Scenario: Known slug
- **WHEN** the category feed is asked for `sports`
- **THEN** it returns the sports articles, fetching them from newsdata.io if no fresh copy is cached

#### Scenario: Different letter case
- **WHEN** `Sports` is requested after `sports` was cached
- **THEN** the same cache entry is used, and any refresh sends `category=sports` upstream

#### Scenario: Unknown value
- **WHEN** the category feed is asked for `politics`, `../x` or `ſports` (long s, which lowercasing does not turn into `s`)
- **THEN** it reports "not found", makes no newsdata.io request and stores nothing

### Requirement: Upstream request
To refresh a category, the service SHALL send `GET https://newsdata.io/api/1/news` with query parameters `category=<slug>` and `language=en,ru`, and the API key in the `X-ACCESS-KEY` request header. The request SHALL NOT carry an `apikey` query parameter.

#### Scenario: Request shape
- **WHEN** the feed refreshes `health`
- **THEN** newsdata.io receives a GET whose decoded query is `category=health` and `language=en,ru`, whose `X-ACCESS-KEY` header equals the configured key, and whose URL contains no `apikey`

### Requirement: API key confidentiality
The newsdata.io API key SHALL NOT appear in any request URL, log record, error message or HTTP response produced by the service.

#### Scenario: Upstream error is logged
- **WHEN** a refresh fails with HTTP 401 and the failure is logged
- **THEN** no log record and no client response contains the API key

### Requirement: Upstream failure detection
A refresh SHALL fail when newsdata.io answers with a status other than 200, when the body is not valid JSON, when the body's `status` field is not `success`, or when no response arrives within 10 seconds.

#### Scenario: Error status
- **WHEN** newsdata.io answers 401, 429 or 500
- **THEN** the refresh fails

#### Scenario: Malformed body
- **WHEN** newsdata.io answers 200 with a body that is not valid JSON, or with `{"status":"error","results":{...}}`
- **THEN** the refresh fails

#### Scenario: Slow upstream
- **WHEN** newsdata.io does not answer within 10 seconds
- **THEN** the refresh fails

#### Scenario: Empty result
- **WHEN** newsdata.io answers 200 with `{"status":"success","results":[]}`
- **THEN** the refresh succeeds with zero articles

### Requirement: One-hour cache
A successful refresh SHALL be cached per category. For one hour after that refresh, requests for the category SHALL be served from the cache without contacting newsdata.io. The first request after the hour SHALL trigger a refresh.

#### Scenario: Fresh copy
- **WHEN** `business` was refreshed 59 minutes ago and is requested again
- **THEN** the cached articles are returned and newsdata.io is not contacted

#### Scenario: Expired copy
- **WHEN** `business` was refreshed 61 minutes ago and is requested again
- **THEN** newsdata.io is contacted and, on success, the new articles are returned and cached

### Requirement: Stale copy on refresh failure
When a refresh fails and an earlier copy of the category exists, the service SHALL return that copy, whatever its age, and log a warning. When no earlier copy exists, the failure SHALL be reported so the category page answers 500. A later request SHALL try to refresh again.

#### Scenario: Outage with a cached copy
- **WHEN** the `food` copy is 3 hours old and newsdata.io answers 500
- **THEN** the 3-hour-old articles are returned, the category page is 200 and a warning is logged

#### Scenario: Outage without a cached copy
- **WHEN** `food` has never been fetched and newsdata.io answers 500
- **THEN** the category page is 500 with a generic body

#### Scenario: Recovery
- **WHEN** a stale copy was served after a failure and newsdata.io is healthy on the next request
- **THEN** that request refreshes the cache and returns the new articles

### Requirement: Concurrent access
The category feed SHALL be safe for concurrent use. Concurrent requests that need a refresh of the same category SHALL share one newsdata.io request, and the refresh SHALL NOT be cancelled because one waiting client disconnects.

#### Scenario: Cold category under load
- **WHEN** 50 requests for an uncached `science` arrive at the same time
- **THEN** newsdata.io receives exactly one request and all 50 get the same articles

#### Scenario: Waiting client disconnects
- **WHEN** the client that triggered a refresh disconnects while other clients wait for the same category
- **THEN** the refresh continues and the other clients get its result

### Requirement: Refresh logging
Each successful newsdata.io refresh SHALL log exactly one Info record `category refreshed` with the category slug, the article count and the upstream call duration, so the owner can track usage against the API quota. Cache hits and failed refreshes SHALL NOT produce this record, and it SHALL NOT contain the API key, URL or response body.

#### Scenario: Cold request then cached hit
- **WHEN** `/category/business` is requested with no cached copy, and then `/category/Business` within the hour
- **THEN** exactly one `category refreshed` record is logged, with `category=business`, the article count and the duration
