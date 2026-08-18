-- +goose Up
CREATE TABLE markets (
    ticker TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Preserve the repository's existing local BTC-USD default while allowing
-- additional tickers to be registered durably through the internal API.
INSERT INTO markets (ticker)
VALUES ('BTC-USD')
ON CONFLICT (ticker) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS markets;
