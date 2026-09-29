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
