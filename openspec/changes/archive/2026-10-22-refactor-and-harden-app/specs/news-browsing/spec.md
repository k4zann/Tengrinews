# Spec Delta

## Purpose

Lets readers browse Tengrinews in a browser: a paginated home page of stored articles, a page per news category, a detail page per stored article, and the static assets those pages use.

## ADDED Requirements

### Requirement: Routes and allowed methods
The service SHALL serve exactly these routes: `/` (home), `/category/{category}`, `/post/{id}` and `/assets/{path}`. Each route SHALL answer GET and HEAD. Any other method on a served route SHALL get 405 with an `Allow` header. Any other path SHALL get 404.

#### Scenario: GET a page
- **WHEN** a client sends `GET /`
- **THEN** the response is 200 with `Content-Type: text/html; charset=utf-8` and the home page body

#### Scenario: HEAD a page
- **WHEN** a client sends `HEAD /`
- **THEN** the response has the same status and `Content-Type` as `GET /` and an empty body

#### Scenario: Disallowed method
- **WHEN** a client sends `POST /`, `PUT /category/business` or `DELETE /post/abc`
- **THEN** the response is 405 and the `Allow` header lists GET and HEAD

#### Scenario: Unknown path
- **WHEN** a client requests `/nope`, `/category/` or `/post/a/b`
- **THEN** the response is 404

### Requirement: Home page pagination
The home page SHALL list stored articles in storage order, 10 per page. The number of pages SHALL be the total number of stored articles divided by 10, rounded up. The page SHALL show a "Previous" link to `?page=N-1` when N > 1 and a "Next" link to `?page=N+1` when N is below the number of pages.

#### Scenario: First page
- **WHEN** 25 articles are stored and a client requests `/`
- **THEN** the page lists articles 1-10, shows a "Next" link to `?page=2` and no "Previous" link

#### Scenario: Last partial page
- **WHEN** 25 articles are stored and a client requests `/?page=3`
- **THEN** the page lists articles 21-25, shows a "Previous" link to `?page=2` and no "Next" link

#### Scenario: Exact multiple of the page size
- **WHEN** 20 articles are stored and a client requests `/?page=2`
- **THEN** the page lists articles 11-20 and shows no "Next" link

#### Scenario: Article links
- **WHEN** the home page lists an article with id `abc123`
- **THEN** its title links to `/post/abc123`

### Requirement: Page parameter fallback
When the `page` query parameter is missing, not an integer, zero or negative, or greater than the number of pages, the home page SHALL render page 1 with status 200.

#### Scenario: Invalid value
- **WHEN** 25 articles are stored and a client requests `/?page=abc`, `/?page=0` or `/?page=-2`
- **THEN** the response is 200 and shows page 1

#### Scenario: Out of range
- **WHEN** 25 articles are stored and a client requests `/?page=4`
- **THEN** the response is 200 and shows page 1

#### Scenario: No stored articles
- **WHEN** no articles are stored and a client requests `/`
- **THEN** the response is 200, the list is empty and neither "Previous" nor "Next" is shown

### Requirement: Post detail page
`/post/{id}` SHALL render the stored article whose article id equals `{id}`: title, published date, first creator when present, image, description and content. When no stored article has that id, the response SHALL be 404.

#### Scenario: Known post
- **WHEN** a stored article has id `abc123` and a client requests `/post/abc123`
- **THEN** the response is 200 and shows that article's title, published date, image, description and content

#### Scenario: Post without creators
- **WHEN** the requested article has an empty creator list
- **THEN** the page is rendered without the "Creator" line

#### Scenario: Unknown post
- **WHEN** no stored article has id `missing` and a client requests `/post/missing`
- **THEN** the response is 404

### Requirement: Category page
`/category/{category}` SHALL lowercase `{category}` and, when the result equals one of the allowed category slugs, render that category's current articles as provided by the category feed. Each listed article SHALL show its image, first category and title. The breadcrumb SHALL show the lowercased slug. Any other value SHALL get 404.

#### Scenario: Known category
- **WHEN** a client requests `/category/business`
- **THEN** the response is 200 and lists the current business articles with the breadcrumb `business`

#### Scenario: Slug with different case
- **WHEN** a client requests `/category/Business` or `/category/BUSINESS`
- **THEN** the response is 200 with the same business articles as `/category/business` and the breadcrumb `business`

#### Scenario: Unknown category
- **WHEN** a client requests `/category/<script>alert(1)</script>` or `/category/politics`
- **THEN** the response is 404 and the body does not contain the requested value

### Requirement: Category navigation
Every page SHALL show a navigation menu with one link per allowed category, in the order business, education, environment, food, health, lifestyle, science, sports, technology. Each link SHALL show the category's display name and point to `/category/<slug>`.

#### Scenario: Menu on every page
- **WHEN** a client renders the home, a category or a post page
- **THEN** the menu contains a link with text `Business` to `/category/business`, and the same for the other eight categories

### Requirement: Output escaping
All values that come from storage, the news feed or the request SHALL be escaped for the HTML context they appear in. Text SHALL never be interpreted as markup, and URLs with unsafe schemes SHALL be neutralized.

#### Scenario: Markup in a title
- **WHEN** an article's title is `<script>alert(1)</script>`
- **THEN** the page contains `&lt;script&gt;alert(1)&lt;/script&gt;` and no `<script>alert(1)` element

#### Scenario: Markup in content
- **WHEN** an article's content contains `<img src=x onerror=alert(1)>`
- **THEN** the post page shows it as text, not as an element

#### Scenario: Unsafe image URL
- **WHEN** an article's image URL is `javascript:alert(1)`
- **THEN** the rendered `src` attribute does not contain `javascript:`

### Requirement: Missing optional article fields
An article with an empty category list, empty creator list or empty image URL SHALL render without error. An empty category list SHALL render an empty category label.

#### Scenario: Empty category list on the home page
- **WHEN** one of the listed articles has no categories
- **THEN** the home page is 200 and that article's category label is empty

#### Scenario: Empty category list on a post page
- **WHEN** the requested article has no categories
- **THEN** the post page is 200 and the category breadcrumb item is empty

### Requirement: Shared page frame
All pages SHALL use one identical `<head>`, header and footer. The tab title SHALL be `Tengrinews`. The copyright line SHALL read `Copyright © <year> All rights reserved | This site was made by Arshat Aitkozha`. Every asset URL in the markup SHALL be absolute (`/assets/...`).

#### Scenario: Same frame on every page
- **WHEN** the home, a category and a post page are rendered
- **THEN** their `<head>`, `<header>` and `<footer>` sections are identical and the title is `Tengrinews`

#### Scenario: No relative asset URLs
- **WHEN** any page is rendered
- **THEN** every stylesheet `href` and image fallback URL starts with `/assets/`

### Requirement: Markup otherwise unchanged
For data without HTML-special characters (`<`, `>`, `&`, `'`, `"`, `+`) and with plain `http(s)` image URLs, each page SHALL render the markup recorded before this change, compared after removing HTML comments and whitespace between tags. The only differences allowed are the listed intended differences that produce the shared page frame.

#### Scenario: Golden comparison
- **WHEN** the home, category and post pages are rendered with the fixture data used to record the pre-change output
- **THEN** each normalized page equals its normalized recording with exactly the listed intended differences applied

#### Scenario: Unlisted drift
- **WHEN** a page's markup differs from its recording in any way that is not a listed intended difference
- **THEN** the comparison fails

### Requirement: Error responses do not leak internals
When a page cannot be produced because of a storage, feed or rendering failure, the response SHALL be 500 with a generic body. Response bodies for 404, 405 and 500 SHALL NOT contain internal error text, stack traces, request input or partial page markup.

#### Scenario: Storage failure
- **WHEN** the article store returns an error while rendering `/`
- **THEN** the response is 500, the body is a generic error message and does not contain the error text

#### Scenario: Rendering failure
- **WHEN** rendering a page fails partway
- **THEN** the response is 500 and contains none of the partially rendered markup

#### Scenario: Handler panic
- **WHEN** a request handler panics
- **THEN** that request gets 500 and the service keeps serving other requests

### Requirement: Static assets
`/assets/{path}` SHALL serve the file at `{path}` from the bundled asset set (CSS, fonts, images). Requests for a directory or a missing file SHALL get 404. Directory listings SHALL never be served.

#### Scenario: Existing file
- **WHEN** a client requests `/assets/css/style.css`
- **THEN** the response is 200 with a CSS content type and the file's bytes

#### Scenario: Directory request
- **WHEN** a client requests `/assets/` or `/assets/css/`
- **THEN** the response is 404 and contains no file listing

#### Scenario: Missing file
- **WHEN** a client requests `/assets/css/owl.carousel.min.css`
- **THEN** the response is 404
