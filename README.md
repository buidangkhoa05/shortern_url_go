# URL Shortener

A small HTTP API that shortens URLs, redirects short codes to their
original URL, supports optional expiration, and tracks click counts.

## Requirements

- Go 1.26+
- Docker (for local Postgres)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI:
  `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

## Setup

1. Copy the example env file and adjust if needed:
   ```bash
   cp .env.example .env
   ```
2. Start Postgres:
   ```bash
   docker-compose up -d
   ```
3. Apply migrations:
   ```bash
   migrate -path migrations -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" up
   ```
4. Run the server:
   ```bash
   go run ./cmd/api
   ```

## API

- `POST /api/v1/links` — `{"long_url": "...", "expires_at": "2026-12-31T00:00:00Z"}` (optional `expires_at`) → `{"short_code", "short_url", "expires_at"}`
- `GET /:code` — `302` redirect to the long URL, `404` if unknown, `410` if expired
- `GET /api/v1/links/:code/stats` — `{"short_code", "long_url", "created_at", "expires_at", "click_count"}`

## Testing

```bash
go test ./...                                    # unit tests
DATABASE_URL="postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" \
  go test -tags integration ./...                # + Postgres integration tests
```

If `make` is available: `make test` and `make test-integration` (set `DATABASE_URL` in your shell first).
