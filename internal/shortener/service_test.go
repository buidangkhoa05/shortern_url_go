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
	if link.ShortCode == nil {
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
		link, err := svc.Resolve(context.Background(), *created.ShortCode)
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
		_, err = svc.Resolve(context.Background(), *expired.ShortCode)
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

	stats, err := svc.GetStats(context.Background(), *created.ShortCode)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.ClickCount != 2 {
		t.Errorf("ClickCount = %d, want 2", stats.ClickCount)
	}
}
