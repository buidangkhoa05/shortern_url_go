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
