CREATE TABLE links (
    id BIGSERIAL PRIMARY KEY,
    long_url TEXT NOT NULL,
    short_code TEXT UNIQUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_links_short_code ON links (short_code);
