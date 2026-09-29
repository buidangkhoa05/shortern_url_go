# URL Shortener MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go HTTP API that shortens URLs, redirects short codes to their long URL, supports optional expiration, and tracks click counts.

**Architecture:** Layered service: Gin HTTP handlers → a `Service` with business logic → a `Repository` interface backed by Postgres (via pgx). Short codes are base62 encodings of the auto-increment row id.

**Tech Stack:** Go 1.26, Gin, PostgreSQL via `jackc/pgx/v5`, `golang-migrate` for schema migrations, `joho/godotenv` + `kelseyhightower/envconfig` for configuration, Docker Compose for local Postgres.

## Global Constraints

- Go 1.26, single module, no `pkg/` directory — application code lives under `internal/`.
- HTTP framework: Gin (`github.com/gin-gonic/gin`).
- Database access via `github.com/jackc/pgx/v5` (`pgxpool`) — plain SQL, no ORM, no code generation.
- Schema migrations via `golang-migrate`, SQL files under `migrations/`.
- Config loaded via `github.com/joho/godotenv` (optional `.env`) + `github.com/kelseyhightower/envconfig` (env vars into a struct).
- Local Postgres runs via `docker-compose.yml`.
- Short codes are base62 (alphabet `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`) encodings of the row's auto-increment `id`.
- `click_events` is an append-only table — no counter column on `links`.
- Repository layer returns sentinel errors (`ErrNotFound`); no HTTP concerns (status codes, `gin.Context`) below the handler layer.
- Click recording on redirect is asynchronous (fired in a goroutine) so it doesn't add latency to the redirect response.
- Integration tests that require a live Postgres are gated behind a `//go:build integration` tag and skip if `DATABASE_URL` isn't set.
- Module path: `shortern-url-go` (local module name, matches the repo folder).

---

### Task 1: Project Scaffolding & Database Schema

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `migrations/000001_create_links.up.sql`
- Create: `migrations/000001_create_links.down.sql`
- Create: `migrations/000002_create_click_events.up.sql`
- Create: `migrations/000002_create_click_events.down.sql`
- Create: `Makefile`

**Interfaces:**
- Produces: the `links` and `click_events` tables that Task 6's `PostgresRepository` reads/writes, and the `DATABASE_URL` connection string format `postgres://user:pass@host:port/dbname?sslmode=disable` used everywhere Postgres is referenced.

- [ ] **Step 1: Initialize the Go module**

Run: `go mod init shortern-url-go`
Expected: creates `go.mod` with `module shortern-url-go` and a `go 1.26` directive.

- [ ] **Step 2: Write `.gitignore`**

```gitignore
/bin/
.env
*.log
```

- [ ] **Step 3: Write `docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: shortener
      POSTGRES_PASSWORD: shortener
      POSTGRES_DB: shortener
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

- [ ] **Step 4: Write `.env.example`**

```dotenv
PORT=8080
DATABASE_URL=postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable
BASE_URL=http://localhost:8080
```

- [ ] **Step 5: Write the `links` migration**

`migrations/000001_create_links.up.sql`:
```sql
CREATE TABLE links (
    id BIGSERIAL PRIMARY KEY,
    long_url TEXT NOT NULL,
    short_code TEXT UNIQUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_links_short_code ON links (short_code);
```

`migrations/000001_create_links.down.sql`:
```sql
DROP TABLE links;
```

- [ ] **Step 6: Write the `click_events` migration**

`migrations/000002_create_click_events.up.sql`:
```sql
CREATE TABLE click_events (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES links(id),
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_click_events_link_id ON click_events (link_id);
```

`migrations/000002_create_click_events.down.sql`:
```sql
DROP TABLE click_events;
```

- [ ] **Step 7: Write the `Makefile`**

```makefile
.PHONY: run build test test-integration docker-up docker-down migrate-up migrate-down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

test-integration:
	go test -tags integration ./...

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down
```

Note: `make` isn't preinstalled on Windows. If unavailable, run the commands inside each target directly (they're documented in `README.md` in Task 9).

- [ ] **Step 8: Install the `migrate` CLI (once, if not already installed)**

Run: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
Expected: installs a `migrate` binary to `$(go env GOPATH)/bin`. Ensure that directory is on `PATH`.

- [ ] **Step 9: Start Postgres and apply migrations**

```bash
docker-compose up -d
# wait a few seconds for Postgres to accept connections
migrate -path migrations -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" up
```
Expected: `1/u create_links (...)` then `2/u create_click_events (...)` printed, no errors.

- [ ] **Step 10: Verify the schema and roll back to confirm `down` migrations work**

```bash
docker exec -it $(docker-compose ps -q postgres) psql -U shortener -d shortener -c "\dt"
migrate -path migrations -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" down 2
migrate -path migrations -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" up
```
Expected: `\dt` lists `links` and `click_events` before the rollback; `down 2` drops both tables without error; the final `up` recreates them, leaving the database ready for later tasks.

- [ ] **Step 11: Commit**

```bash
git add go.mod .gitignore docker-compose.yml .env.example migrations Makefile
git commit -m "chore: scaffold project, docker-compose, and DB migrations"
```

---

### Task 2: Configuration Loading

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing (reads process environment / `.env`).
- Produces: `config.Config{Port, DatabaseURL, BaseURL string}` and `config.Load() (Config, error)`, used by `cmd/api/main.go` in Task 8.

- [ ] **Step 1: Add dependencies**

Run: `go get github.com/joho/godotenv github.com/kelseyhightower/envconfig`

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:
```go
package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user:pass@localhost:5432/db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want default %q", cfg.BaseURL, "http://localhost:8080")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error when DATABASE_URL is unset, got nil")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/config/...`
Expected: FAIL — `config.Load` undefined (package doesn't exist yet).

- [ ] **Step 4: Write the implementation**

`internal/config/config.go`:
```go
package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	BaseURL     string `envconfig:"BASE_URL" default:"http://localhost:8080"`
}

func Load() (Config, error) {
	_ = godotenv.Load() // .env is optional; ignore if absent

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS (both `TestLoad` and `TestLoad_MissingRequired`). If `TestLoad_MissingRequired` fails because `DATABASE_URL` is already set in your shell, unset it first and rerun.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config
git commit -m "feat: add environment-based configuration loading"
```

---

### Task 3: Base62 Short Code Encoding

**Files:**
- Create: `internal/shortener/base62.go`
- Test: `internal/shortener/base62_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `shortener.EncodeBase62(id int64) string`, used by `Service.CreateLink` in Task 5.

- [ ] **Step 1: Write the failing test**

`internal/shortener/base62_test.go`:
```go
package shortener

import "testing"

func TestEncodeBase62(t *testing.T) {
	tests := []struct {
		id   int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{61, "Z"},
		{62, "10"},
		{125, "21"},
	}
	for _, tt := range tests {
		if got := EncodeBase62(tt.id); got != tt.want {
			t.Errorf("EncodeBase62(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/shortener/... -run TestEncodeBase62`
Expected: FAIL — build error, `EncodeBase62` undefined.

- [ ] **Step 3: Write the implementation**

`internal/shortener/base62.go`:
```go
package shortener

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func EncodeBase62(id int64) string {
	if id == 0 {
		return string(base62Alphabet[0])
	}
	var buf []byte
	for id > 0 {
		remainder := id % 62
		buf = append([]byte{base62Alphabet[remainder]}, buf...)
		id /= 62
	}
	return string(buf)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/shortener/... -run TestEncodeBase62`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shortener/base62.go internal/shortener/base62_test.go
git commit -m "feat: add base62 short code encoding"
```

---

### Task 4: Domain Model & Repository Interface

**Files:**
- Create: `internal/shortener/model.go`
- Create: `internal/shortener/repository.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `shortener.Link{ID int64, LongURL string, ShortCode string, ExpiresAt *time.Time, CreatedAt time.Time}`
  - `shortener.ErrNotFound error`
  - `shortener.Repository` interface:
    ```go
    type Repository interface {
        CreateLink(ctx context.Context, longURL string, expiresAt *time.Time) (Link, error)
        SetShortCode(ctx context.Context, id int64, shortCode string) error
        GetByShortCode(ctx context.Context, shortCode string) (Link, error)
        RecordClick(ctx context.Context, linkID int64) error
        CountClicks(ctx context.Context, linkID int64) (int64, error)
    }
    ```
  Task 5 (`Service`, fake repository) and Task 6 (`PostgresRepository`) both implement/consume this exact interface.

- [ ] **Step 1: Write the domain model**

`internal/shortener/model.go`:
```go
package shortener

import "time"

type Link struct {
	ID        int64
	LongURL   string
	ShortCode string
	ExpiresAt *time.Time
	CreatedAt time.Time
}
```

- [ ] **Step 2: Write the repository interface**

`internal/shortener/repository.go`:
```go
package shortener

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("shortener: link not found")

type Repository interface {
	CreateLink(ctx context.Context, longURL string, expiresAt *time.Time) (Link, error)
	SetShortCode(ctx context.Context, id int64, shortCode string) error
	GetByShortCode(ctx context.Context, shortCode string) (Link, error)
	RecordClick(ctx context.Context, linkID int64) error
	CountClicks(ctx context.Context, linkID int64) (int64, error)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/shortener/model.go internal/shortener/repository.go
git commit -m "feat: add Link domain model and Repository interface"
```

---

### Task 5: Service Layer (business logic)

**Files:**
- Create: `internal/shortener/service.go`
- Create: `internal/shortener/fake_repository_test.go`
- Test: `internal/shortener/service_test.go`

**Interfaces:**
- Consumes: `shortener.Repository`, `shortener.Link`, `shortener.ErrNotFound`, `shortener.EncodeBase62` (Tasks 3–4).
- Produces:
  - `shortener.ErrExpired error`
  - `shortener.CreateLinkInput{LongURL string, ExpiresAt *time.Time}`
  - `shortener.LinkStats{Link, ClickCount int64}`
  - `shortener.NewService(repo Repository) *Service`
  - `(*Service).CreateLink(ctx, CreateLinkInput) (Link, error)`
  - `(*Service).Resolve(ctx, shortCode string) (Link, error)`
  - `(*Service).RecordClick(ctx, linkID int64) error`
  - `(*Service).GetStats(ctx, shortCode string) (LinkStats, error)`
  Task 7's `Handler` (via a `LinkService` interface) consumes these exact method signatures.

- [ ] **Step 1: Write the in-memory fake repository (test helper)**

`internal/shortener/fake_repository_test.go`:
```go
package shortener

import (
	"context"
	"time"
)

type fakeRepository struct {
	byID       map[int64]Link
	codeToID   map[string]int64
	nextID     int64
	clickCount map[int64]int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:       make(map[int64]Link),
		codeToID:   make(map[string]int64),
		clickCount: make(map[int64]int64),
	}
}

func (f *fakeRepository) CreateLink(ctx context.Context, longURL string, expiresAt *time.Time) (Link, error) {
	f.nextID++
	link := Link{ID: f.nextID, LongURL: longURL, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	f.byID[link.ID] = link
	return link, nil
}

func (f *fakeRepository) SetShortCode(ctx context.Context, id int64, shortCode string) error {
	link, ok := f.byID[id]
	if !ok {
		return ErrNotFound
	}
	link.ShortCode = shortCode
	f.byID[id] = link
	f.codeToID[shortCode] = id
	return nil
}

func (f *fakeRepository) GetByShortCode(ctx context.Context, shortCode string) (Link, error) {
	id, ok := f.codeToID[shortCode]
	if !ok {
		return Link{}, ErrNotFound
	}
	return f.byID[id], nil
}

func (f *fakeRepository) RecordClick(ctx context.Context, linkID int64) error {
	f.clickCount[linkID]++
	return nil
}

func (f *fakeRepository) CountClicks(ctx context.Context, linkID int64) (int64, error) {
	return f.clickCount[linkID], nil
}
```

- [ ] **Step 2: Write the failing tests**

`internal/shortener/service_test.go`:
```go
package shortener

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_CreateLink(t *testing.T) {
	svc := NewService(newFakeRepository())

	link, err := svc.CreateLink(context.Background(), CreateLinkInput{LongURL: "https://example.com"})
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if link.ShortCode == "" {
		t.Error("CreateLink() did not set ShortCode")
	}
	if link.LongURL != "https://example.com" {
		t.Errorf("LongURL = %q, want %q", link.LongURL, "https://example.com")
	}
}

func TestService_Resolve(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	created, err := svc.CreateLink(context.Background(), CreateLinkInput{LongURL: "https://example.com"})
	if err != nil {
		t.Fatalf("setup CreateLink() error = %v", err)
	}

	t.Run("found", func(t *testing.T) {
		link, err := svc.Resolve(context.Background(), created.ShortCode)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if link.LongURL != "https://example.com" {
			t.Errorf("LongURL = %q, want %q", link.LongURL, "https://example.com")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Resolve(context.Background(), "doesnotexist")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Resolve() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		expired, err := svc.CreateLink(context.Background(), CreateLinkInput{LongURL: "https://old.com", ExpiresAt: &past})
		if err != nil {
			t.Fatalf("setup CreateLink() error = %v", err)
		}
		_, err = svc.Resolve(context.Background(), expired.ShortCode)
		if !errors.Is(err, ErrExpired) {
			t.Errorf("Resolve() error = %v, want ErrExpired", err)
		}
	})
}

func TestService_GetStats(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	created, err := svc.CreateLink(context.Background(), CreateLinkInput{LongURL: "https://example.com"})
	if err != nil {
		t.Fatalf("setup CreateLink() error = %v", err)
	}

	if err := svc.RecordClick(context.Background(), created.ID); err != nil {
		t.Fatalf("RecordClick() error = %v", err)
	}
	if err := svc.RecordClick(context.Background(), created.ID); err != nil {
		t.Fatalf("RecordClick() error = %v", err)
	}

	stats, err := svc.GetStats(context.Background(), created.ShortCode)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.ClickCount != 2 {
		t.Errorf("ClickCount = %d, want 2", stats.ClickCount)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/shortener/... -run TestService`
Expected: FAIL — build error, `NewService`/`CreateLinkInput`/`ErrExpired`/`LinkStats` undefined.

- [ ] **Step 4: Write the implementation**

`internal/shortener/service.go`:
```go
package shortener

import (
	"context"
	"errors"
	"time"
)

var ErrExpired = errors.New("shortener: link expired")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CreateLinkInput struct {
	LongURL   string
	ExpiresAt *time.Time
}

func (s *Service) CreateLink(ctx context.Context, input CreateLinkInput) (Link, error) {
	link, err := s.repo.CreateLink(ctx, input.LongURL, input.ExpiresAt)
	if err != nil {
		return Link{}, err
	}
	code := EncodeBase62(link.ID)
	if err := s.repo.SetShortCode(ctx, link.ID, code); err != nil {
		return Link{}, err
	}
	link.ShortCode = code
	return link, nil
}

func (s *Service) Resolve(ctx context.Context, shortCode string) (Link, error) {
	link, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return Link{}, err
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return Link{}, ErrExpired
	}
	return link, nil
}

func (s *Service) RecordClick(ctx context.Context, linkID int64) error {
	return s.repo.RecordClick(ctx, linkID)
}

type LinkStats struct {
	Link
	ClickCount int64
}

func (s *Service) GetStats(ctx context.Context, shortCode string) (LinkStats, error) {
	link, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return LinkStats{}, err
	}
	count, err := s.repo.CountClicks(ctx, link.ID)
	if err != nil {
		return LinkStats{}, err
	}
	return LinkStats{Link: link, ClickCount: count}, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/shortener/... -run TestService -v`
Expected: PASS for `TestService_CreateLink`, `TestService_Resolve` (all subtests), `TestService_GetStats`.

- [ ] **Step 6: Commit**

```bash
git add internal/shortener/service.go internal/shortener/service_test.go internal/shortener/fake_repository_test.go
git commit -m "feat: add Service business logic for link creation, resolution, and stats"
```

---

### Task 6: Postgres Repository Implementation

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/shortener/repository_postgres.go`
- Test: `internal/shortener/repository_postgres_integration_test.go`

**Interfaces:**
- Consumes: `shortener.Repository`, `shortener.Link`, `shortener.ErrNotFound` (Task 4); the `links`/`click_events` schema (Task 1).
- Produces:
  - `database.NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)`, used by `cmd/api/main.go` in Task 8.
  - `shortener.NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository`, satisfying `shortener.Repository`, used by `cmd/api/main.go` in Task 8.

- [ ] **Step 1: Add the pgx dependency**

Run: `go get github.com/jackc/pgx/v5`

- [ ] **Step 2: Write the connection pool helper**

`internal/database/database.go`:
```go
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 3: Write the Postgres repository implementation**

`internal/shortener/repository_postgres.go`:
```go
package shortener

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateLink(ctx context.Context, longURL string, expiresAt *time.Time) (Link, error) {
	var link Link
	err := r.pool.QueryRow(ctx,
		`INSERT INTO links (long_url, expires_at) VALUES ($1, $2)
		 RETURNING id, long_url, short_code, expires_at, created_at`,
		longURL, expiresAt,
	).Scan(&link.ID, &link.LongURL, &link.ShortCode, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		return Link{}, err
	}
	return link, nil
}

func (r *PostgresRepository) SetShortCode(ctx context.Context, id int64, shortCode string) error {
	_, err := r.pool.Exec(ctx, `UPDATE links SET short_code = $1 WHERE id = $2`, shortCode, id)
	return err
}

func (r *PostgresRepository) GetByShortCode(ctx context.Context, shortCode string) (Link, error) {
	var link Link
	err := r.pool.QueryRow(ctx,
		`SELECT id, long_url, short_code, expires_at, created_at FROM links WHERE short_code = $1`,
		shortCode,
	).Scan(&link.ID, &link.LongURL, &link.ShortCode, &link.ExpiresAt, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, err
	}
	return link, nil
}

func (r *PostgresRepository) RecordClick(ctx context.Context, linkID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO click_events (link_id) VALUES ($1)`, linkID)
	return err
}

func (r *PostgresRepository) CountClicks(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM click_events WHERE link_id = $1`, linkID).Scan(&count)
	return count, err
}
```

- [ ] **Step 4: Write the integration test**

`internal/shortener/repository_postgres_integration_test.go`:
```go
//go:build integration

package shortener

import (
	"context"
	"os"
	"testing"

	"shortern-url-go/internal/database"
)

func TestPostgresRepository_CreateAndFetch(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := NewPostgresRepository(pool)

	link, err := repo.CreateLink(ctx, "https://example.com", nil)
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	if err := repo.SetShortCode(ctx, link.ID, "abc123"); err != nil {
		t.Fatalf("SetShortCode() error = %v", err)
	}

	fetched, err := repo.GetByShortCode(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetByShortCode() error = %v", err)
	}
	if fetched.LongURL != "https://example.com" {
		t.Errorf("LongURL = %q, want %q", fetched.LongURL, "https://example.com")
	}

	if err := repo.RecordClick(ctx, fetched.ID); err != nil {
		t.Fatalf("RecordClick() error = %v", err)
	}
	count, err := repo.CountClicks(ctx, fetched.ID)
	if err != nil {
		t.Fatalf("CountClicks() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CountClicks() = %d, want 1", count)
	}
}
```

- [ ] **Step 5: Run the integration test against the Postgres started in Task 1**

Run (bash):
```bash
DATABASE_URL="postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" \
  go test -tags integration ./internal/shortener/... -run TestPostgresRepository -v
```
Expected: PASS. If it fails to connect, confirm `docker-compose up -d` is running (Task 1, Step 9).

- [ ] **Step 6: Run the full non-integration suite to confirm nothing broke**

Run: `go test ./...`
Expected: PASS (the integration test is skipped by default since it requires the `integration` build tag).

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/database internal/shortener/repository_postgres.go internal/shortener/repository_postgres_integration_test.go
git commit -m "feat: add Postgres-backed Repository implementation"
```

---

### Task 7: HTTP Handlers

**Files:**
- Create: `internal/shortener/handler.go`
- Test: `internal/shortener/handler_test.go`

**Interfaces:**
- Consumes: `shortener.Link`, `shortener.LinkStats`, `shortener.CreateLinkInput`, `shortener.ErrNotFound`, `shortener.ErrExpired` (Tasks 4–5). Defines its own `LinkService` interface so `*Service` (Task 5) is consumed structurally, without a hard dependency.
- Produces:
  - `shortener.LinkService` interface (subset of `*Service`'s methods).
  - `shortener.NewHandler(service LinkService, baseURL string) *Handler`
  - `(*Handler).RegisterRoutes(r *gin.Engine)`, used by `cmd/api/main.go` in Task 8.

- [ ] **Step 1: Add the Gin dependency**

Run: `go get github.com/gin-gonic/gin`

- [ ] **Step 2: Write the failing tests**

`internal/shortener/handler_test.go`:
```go
package shortener

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	createLinkFn  func(ctx context.Context, input CreateLinkInput) (Link, error)
	resolveFn     func(ctx context.Context, shortCode string) (Link, error)
	recordClickFn func(ctx context.Context, linkID int64) error
	getStatsFn    func(ctx context.Context, shortCode string) (LinkStats, error)
}

func (f *fakeService) CreateLink(ctx context.Context, input CreateLinkInput) (Link, error) {
	return f.createLinkFn(ctx, input)
}
func (f *fakeService) Resolve(ctx context.Context, shortCode string) (Link, error) {
	return f.resolveFn(ctx, shortCode)
}
func (f *fakeService) RecordClick(ctx context.Context, linkID int64) error {
	if f.recordClickFn != nil {
		return f.recordClickFn(ctx, linkID)
	}
	return nil
}
func (f *fakeService) GetStats(ctx context.Context, shortCode string) (LinkStats, error) {
	return f.getStatsFn(ctx, shortCode)
}

func newTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func TestHandler_CreateLink(t *testing.T) {
	svc := &fakeService{
		createLinkFn: func(ctx context.Context, input CreateLinkInput) (Link, error) {
			return Link{ID: 1, LongURL: input.LongURL, ShortCode: "1"}, nil
		},
	}
	h := NewHandler(svc, "http://localhost:8080")
	router := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{"long_url": "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp createLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ShortURL != "http://localhost:8080/1" {
		t.Errorf("ShortURL = %q, want %q", resp.ShortURL, "http://localhost:8080/1")
	}
}

func TestHandler_Redirect(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{ID: 1, LongURL: "https://example.com", ShortCode: shortCode}, nil
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
		}
		if loc := w.Header().Get("Location"); loc != "https://example.com" {
			t.Errorf("Location = %q, want %q", loc, "https://example.com")
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{}, ErrNotFound
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{}, ErrExpired
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/old", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusGone {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusGone)
		}
	})
}

func TestHandler_Stats(t *testing.T) {
	created := time.Now()
	svc := &fakeService{
		getStatsFn: func(ctx context.Context, shortCode string) (LinkStats, error) {
			return LinkStats{
				Link:       Link{ShortCode: shortCode, LongURL: "https://example.com", CreatedAt: created},
				ClickCount: 5,
			}, nil
		},
	}
	h := NewHandler(svc, "http://localhost:8080")
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/abc/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ClickCount != 5 {
		t.Errorf("ClickCount = %d, want 5", resp.ClickCount)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/shortener/... -run TestHandler`
Expected: FAIL — build error, `Handler`/`NewHandler`/`createLinkResponse`/`statsResponse` undefined.

- [ ] **Step 4: Write the implementation**

`internal/shortener/handler.go`:
```go
package shortener

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type LinkService interface {
	CreateLink(ctx context.Context, input CreateLinkInput) (Link, error)
	Resolve(ctx context.Context, shortCode string) (Link, error)
	RecordClick(ctx context.Context, linkID int64) error
	GetStats(ctx context.Context, shortCode string) (LinkStats, error)
}

type Handler struct {
	service LinkService
	baseURL string
}

func NewHandler(service LinkService, baseURL string) *Handler {
	return &Handler{service: service, baseURL: baseURL}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/api/v1/links", h.CreateLink)
	r.GET("/api/v1/links/:code/stats", h.Stats)
	r.GET("/:code", h.Redirect)
}

type createLinkRequest struct {
	LongURL   string     `json:"long_url" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type createLinkResponse struct {
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *Handler) CreateLink(c *gin.Context) {
	var req createLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.service.CreateLink(c.Request.Context(), CreateLinkInput{
		LongURL:   req.LongURL,
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	c.JSON(http.StatusCreated, createLinkResponse{
		ShortCode: link.ShortCode,
		ShortURL:  h.baseURL + "/" + link.ShortCode,
		ExpiresAt: link.ExpiresAt,
	})
}

func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")
	link, err := h.service.Resolve(c.Request.Context(), code)
	switch {
	case errors.Is(err, ErrNotFound):
		c.Status(http.StatusNotFound)
		return
	case errors.Is(err, ErrExpired):
		c.Status(http.StatusGone)
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve link"})
		return
	}

	go func() {
		if err := h.service.RecordClick(context.Background(), link.ID); err != nil {
			log.Printf("failed to record click for link %d: %v", link.ID, err)
		}
	}()

	c.Redirect(http.StatusFound, link.LongURL)
}

type statsResponse struct {
	ShortCode  string     `json:"short_code"`
	LongURL    string     `json:"long_url"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ClickCount int64      `json:"click_count"`
}

func (h *Handler) Stats(c *gin.Context) {
	code := c.Param("code")
	stats, err := h.service.GetStats(c.Request.Context(), code)
	switch {
	case errors.Is(err, ErrNotFound):
		c.Status(http.StatusNotFound)
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, statsResponse{
		ShortCode:  stats.ShortCode,
		LongURL:    stats.LongURL,
		CreatedAt:  stats.CreatedAt,
		ExpiresAt:  stats.ExpiresAt,
		ClickCount: stats.ClickCount,
	})
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/shortener/... -run TestHandler -v`
Expected: PASS for `TestHandler_CreateLink`, `TestHandler_Redirect` (all subtests), `TestHandler_Stats`.

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/shortener/handler.go internal/shortener/handler_test.go
git commit -m "feat: add Gin HTTP handlers for create, redirect, and stats endpoints"
```

---

### Task 8: Server Wiring & Entry Point

**Files:**
- Create: `internal/httpserver/server.go`
- Create: `cmd/api/main.go`

**Interfaces:**
- Consumes: `config.Load` (Task 2), `database.NewPool` (Task 6), `shortener.NewPostgresRepository`, `shortener.NewService`, `shortener.NewHandler`, `(*Handler).RegisterRoutes` (Tasks 5–7).
- Produces: the running binary at `cmd/api`; nothing else consumes this package.

- [ ] **Step 1: Write the HTTP server wrapper**

`internal/httpserver/server.go`:
```go
package httpserver

import (
	"context"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func New(addr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
```

- [ ] **Step 2: Write the entry point**

`cmd/api/main.go`:
```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"shortern-url-go/internal/config"
	"shortern-url-go/internal/database"
	"shortern-url-go/internal/httpserver"
	"shortern-url-go/internal/shortener"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := shortener.NewPostgresRepository(pool)
	service := shortener.NewService(repo)
	handler := shortener.NewHandler(service, cfg.BaseURL)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())
	handler.RegisterRoutes(router)

	srv := httpserver.New(":"+cfg.Port, router)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("listening on :%s", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server exited")
}
```

- [ ] **Step 3: Build the binary**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Manually verify the full flow end-to-end**

With Postgres running (Task 1) and `DATABASE_URL`/`PORT`/`BASE_URL` set (copy `.env.example` to `.env` and adjust if needed):

```bash
go run ./cmd/api
```

In another terminal:
```bash
curl -s -X POST http://localhost:8080/api/v1/links \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://example.com"}'
# => {"short_code":"1","short_url":"http://localhost:8080/1"}

curl -s -i http://localhost:8080/1
# => HTTP/1.1 302 Found, Location: https://example.com

curl -s http://localhost:8080/api/v1/links/1/stats
# => {"short_code":"1","long_url":"https://example.com","created_at":"...","click_count":1}
```

Expected: the create call returns a short code, the redirect returns `302` to the original URL, and stats show `click_count` incremented after the redirect (allow a moment for the async click write).

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver cmd/api
git commit -m "feat: wire config, database, and handlers into a runnable HTTP server"
```

---

### Task 9: README

**Files:**
- Create: `README.md`

**Interfaces:**
- Consumes: nothing — documents Tasks 1–8's commands.
- Produces: nothing consumed by other tasks.

- [ ] **Step 1: Write the README**

`README.md`:
```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add setup and usage instructions"
```
