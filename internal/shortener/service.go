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
