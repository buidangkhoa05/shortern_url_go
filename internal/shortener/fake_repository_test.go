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
