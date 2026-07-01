-- +goose Up
CREATE TABLE match_logs (
    match_log_id BIGSERIAL PRIMARY KEY,
    ticker TEXT NOT NULL,
    price DOUBLE PRECISION NOT NULL CHECK (price > 0),
    amount DOUBLE PRECISION NOT NULL CHECK (amount > 0),
    quote_amount DOUBLE PRECISION NOT NULL CHECK (quote_amount >= 0),
    maker_order_id TEXT NOT NULL,
    taker_order_id TEXT NOT NULL,
    maker_user_id TEXT NOT NULL,
    taker_user_id TEXT NOT NULL,
    maker_side TEXT NOT NULL CHECK (maker_side IN ('BID', 'ASK')),
    taker_side TEXT NOT NULL CHECK (taker_side IN ('BID', 'ASK')),
    matched_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX match_logs_ticker_matched_at_idx ON match_logs (ticker, matched_at DESC);
CREATE INDEX match_logs_maker_user_matched_at_idx ON match_logs (maker_user_id, matched_at DESC);
CREATE INDEX match_logs_taker_user_matched_at_idx ON match_logs (taker_user_id, matched_at DESC);
CREATE INDEX match_logs_maker_order_id_idx ON match_logs (maker_order_id);
CREATE INDEX match_logs_taker_order_id_idx ON match_logs (taker_order_id);

-- +goose Down
DROP TABLE IF EXISTS match_logs;
