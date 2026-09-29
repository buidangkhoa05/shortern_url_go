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
