# Spec Delta

## Purpose

Defines how the Tengrinews process is configured, started, bounded in time, observed and stopped, and how it accesses MongoDB, so it fails fast on bad setup and never exposes secrets.

## ADDED Requirements

### Requirement: Required configuration
The service SHALL read `NEWSDATA_API_KEY` and `MONGODB_URI` from the environment. If either is missing or empty, it SHALL exit with a non-zero code before listening, logging the names of the missing variables and never their values.

#### Scenario: Missing variable
- **WHEN** the service starts with `MONGODB_URI` set and `NEWSDATA_API_KEY` unset
- **THEN** it exits non-zero without listening and logs an error naming `NEWSDATA_API_KEY`

#### Scenario: Both variables set
- **WHEN** both variables are set and MongoDB is reachable
- **THEN** the service starts listening

### Requirement: Fail-fast startup
The service SHALL verify MongoDB with a ping that must succeed within 10 seconds and SHALL load all page templates before listening. If either fails, it SHALL exit with a non-zero code without listening.

#### Scenario: MongoDB unreachable
- **WHEN** `MONGODB_URI` points to a host that does not answer
- **THEN** the service exits non-zero within about 10 seconds and never listens

#### Scenario: Broken template
- **WHEN** a page template cannot be parsed
- **THEN** the service exits non-zero and never listens

### Requirement: Listen address
The service SHALL listen for HTTP on port 8080 on all interfaces. The port SHALL NOT be configurable.

#### Scenario: Default start
- **WHEN** the service starts successfully
- **THEN** it accepts HTTP connections on port 8080

### Requirement: Self-contained binary
The built binary SHALL contain its page templates and static assets and SHALL serve them correctly whatever the current working directory is.

#### Scenario: Started from another directory
- **WHEN** the binary is started from a directory that has no `ui/` folder
- **THEN** all pages and `/assets/css/style.css` are served normally

### Requirement: Inbound HTTP timeouts
The HTTP server SHALL close a connection whose request headers are not fully received within 5 seconds, SHALL bound reading a whole request to 10 seconds, writing a response to 20 seconds, and keep idle connections for at most 60 seconds.

#### Scenario: Slow headers
- **WHEN** a client opens a connection and sends request headers slower than 5 seconds
- **THEN** the server closes the connection

### Requirement: Outbound call timeouts
Every MongoDB operation SHALL be bounded to 5 seconds and every newsdata.io request to 10 seconds. MongoDB operations made for a request SHALL stop when that request's client disconnects.

#### Scenario: MongoDB hangs
- **WHEN** MongoDB stops answering while `/` is requested
- **THEN** the request gets 500 within about 5 seconds

#### Scenario: Client disconnects
- **WHEN** a client disconnects while its home-page query is running
- **THEN** the MongoDB operation is cancelled

### Requirement: Structured logging
The service SHALL write logs as JSON lines to standard output. It SHALL log one record per HTTP request with method, path without query string, status and duration, and one error record with the underlying error for every 500 response.

#### Scenario: Served request
- **WHEN** `GET /?page=2` is answered with 200
- **THEN** one JSON log record contains method `GET`, path `/`, status `200` and a duration

#### Scenario: Internal failure
- **WHEN** a request fails with 500 because of a storage error
- **THEN** an error-level JSON record contains the storage error text, which the client never sees

### Requirement: Secrets never logged
The newsdata.io API key and the MongoDB connection URI SHALL NOT appear in any log record, including startup, connection-failure and shutdown records.

#### Scenario: Connection failure with credentials in the URI
- **WHEN** the service starts with `MONGODB_URI=mongodb://user:s3cr3t@unreachable:27017` and the ping fails
- **THEN** no log record contains `s3cr3t` or the full URI

### Requirement: Read-only database access
The service SHALL only read from MongoDB: database `news`, collection `articles`, documents shaped `{articles: [article, ...]}`. It SHALL NOT insert, update, delete or create indexes. The README SHALL document the index on `articles.article_id` that operators create by hand.

#### Scenario: Read-only database user
- **WHEN** the service runs with a MongoDB user that has only the `read` role on `news`
- **THEN** every page works and no write or index command is sent

#### Scenario: Index documented
- **WHEN** an operator reads the README
- **THEN** it gives the exact command to create the `articles.article_id` index

### Requirement: Targeted post lookup
The service SHALL find a post with one database query filtered on `articles.article_id` that returns only the matching article, and SHALL NOT load the whole collection into the process.

#### Scenario: Lookup in a large collection
- **WHEN** the collection holds many documents and `/post/abc123` is requested
- **THEN** one query filtered on `articles.article_id = "abc123"` is sent and only the matching article is returned to the service

### Requirement: Graceful shutdown
On SIGINT or SIGTERM the service SHALL stop accepting connections, let in-flight requests finish for up to 10 seconds, then disconnect from MongoDB and exit with code 0.

#### Scenario: In-flight request during shutdown
- **WHEN** SIGTERM arrives while a request is being served
- **THEN** that request completes normally, new connections are refused, and the process exits 0 after disconnecting from MongoDB

#### Scenario: Shutdown deadline
- **WHEN** an in-flight request is still running 10 seconds after SIGTERM
- **THEN** the service stops waiting, disconnects from MongoDB and exits with a non-zero code
