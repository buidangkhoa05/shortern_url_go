# URL Shortener Service — Design

Date: 2026-09-29

## Purpose

A backend HTTP API for shortening URLs: given a long URL, produce a short
code that redirects to it. Supports optional expiration and tracks click
counts per link.

## Scope

In scope:
- Create a short link from a long URL, with an optional expiration timestamp.
- Redirect from a short code to the original long URL.
- Track and expose click counts per short link.

Out of scope (for this iteration):
- Custom user-chosen aliases.
- Authentication / per-user link ownership.
- Rate limiting / abuse prevention.

## Tech choices

- Go 1.26, module-based (`go.mod`).
- HTTP framework: Gin.
- Database: PostgreSQL, accessed via `jackc/pgx/v5` (`pgxpool`), plain SQL
  (no ORM, no code-gen) — the query surface is small enough not to justify
  the extra dependency/tooling.
- Migrations: `golang-migrate`, SQL files under `migrations/`.
- Config: `joho/godotenv` (loads `.env` for local dev) +
  `kelseyhightower/envconfig` (unmarshals env vars into a config struct).
- Local dev environment: `docker-compose.yml` running Postgres.

## Project layout

```
shortern_url_go/
├── cmd/
│   └── api/
│       └── main.go              # wires config, DB, router; starts HTTP server
├── internal/
│   ├── config/                  # env var loading (godotenv + envconfig)
│   ├── httpserver/               # Gin engine setup, middleware, graceful shutdown
│   ├── shortener/
│   │   ├── handler.go            # Gin HTTP handlers (request/response, validation)
│   │   ├── service.go            # business logic: base62 encode, expiration check
│   │   ├── repository.go         # Postgres queries (pgx)
│   │   └── model.go              # domain types (Link, ClickEvent)
│   └── database/                 # pgxpool connection setup
├── migrations/                   # golang-migrate .sql files
├── docker-compose.yml            # Postgres service
├── .env.example
├── Makefile                      # build, run, migrate, test targets
├── go.mod / go.sum
└── README.md
```

Layering: **handler** (HTTP/Gin concerns only) → **service** (business rules,
depends on a repository interface) → **repository** (SQL via pgx). Handlers
never touch SQL; the service never touches `gin.Context`.

## Data model

`links` table:
- `id BIGSERIAL PRIMARY KEY`
- `long_url TEXT NOT NULL`
- `short_code TEXT UNIQUE` — base62 encoding of `id`, backfilled after insert
- `expires_at TIMESTAMPTZ NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

`click_events` table:
- `id BIGSERIAL PRIMARY KEY`
- `link_id BIGINT NOT NULL REFERENCES links(id)`
- `clicked_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Click events are an append-only table rather than a counter column on
`links`, so future analytics (clicks over time) don't require a schema
change — a `COUNT(*)` gives the current total.

## Short code generation

Base62 encoding of the auto-increment DB id (alphabet `[0-9a-zA-Z]`):
1. Insert the row with `short_code` temporarily empty (or NULL) to obtain
   the generated `id`.
2. Encode `id` as base62.
3. `UPDATE` the row to set `short_code`.

This guarantees uniqueness without collision retries and keeps codes short.

## API

- `POST /api/v1/links`
  - Request: `{ "long_url": string, "expires_at": string? (RFC3339) }`
  - Response: `201 { "short_code": string, "short_url": string, "expires_at": string? }`
  - `400` if `long_url` is missing/invalid.

- `GET /:code`
  - `302` redirect to `long_url` if found and not expired.
  - `404` if the code doesn't exist.
  - `410` if the link has expired.
  - On success, records a click asynchronously (goroutine) — does not add
    latency to the redirect response.

- `GET /api/v1/links/:code/stats`
  - Response: `200 { "short_code": string, "long_url": string, "created_at": string, "expires_at": string?, "click_count": number }`
  - `404` if the code doesn't exist.

## Error handling

- Repository returns sentinel errors (e.g. `ErrNotFound`) — no HTTP
  concerns leak into the repository layer.
- The service maps domain errors to meaningful results; the handler maps
  those to HTTP status codes.
- Startup failures (bad config, DB unreachable) are fatal — the service
  cannot do anything useful without them.

## Testing

- Table-driven unit tests for `service.go` (base62 encoding, expiration
  logic) using a mocked repository interface.
- Repository tests run against a real Postgres via `docker-compose`,
  gated behind an `integration` build tag (`//go:build integration`) so
  they're opt-in and don't require Postgres for a plain `go test ./...`.
