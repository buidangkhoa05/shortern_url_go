CREATE TABLE click_events (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES links(id),
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_click_events_link_id ON click_events (link_id);
