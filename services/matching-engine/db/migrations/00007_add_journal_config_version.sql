-- +goose Up
ALTER TABLE order_journal
    ADD COLUMN market_config_version BIGINT NOT NULL DEFAULT 1
        CHECK (market_config_version > 0);

-- +goose Down
ALTER TABLE order_journal
    DROP COLUMN IF EXISTS market_config_version;
