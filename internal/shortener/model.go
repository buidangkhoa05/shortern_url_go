package shortener

import "time"

type Link struct {
	ID        int64
	LongURL   string
	ShortCode string
	ExpiresAt *time.Time
	CreatedAt time.Time
}
