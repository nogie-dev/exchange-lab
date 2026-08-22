-- +goose Up
ALTER TABLE markets
    ADD COLUMN price_scale SMALLINT NOT NULL DEFAULT 8
        CHECK (price_scale BETWEEN 0 AND 18),
    ADD COLUMN quantity_scale SMALLINT NOT NULL DEFAULT 8
        CHECK (quantity_scale BETWEEN 0 AND 18),
    ADD COLUMN quote_scale SMALLINT NOT NULL DEFAULT 8
        CHECK (quote_scale BETWEEN 0 AND 18),
    ADD COLUMN tick_size_units BIGINT NOT NULL DEFAULT 1
        CHECK (tick_size_units > 0),
    ADD COLUMN lot_size_units BIGINT NOT NULL DEFAULT 1
        CHECK (lot_size_units > 0),
    ADD COLUMN min_price_ticks BIGINT NOT NULL DEFAULT 1
        CHECK (min_price_ticks > 0),
    ADD COLUMN max_price_ticks BIGINT NOT NULL DEFAULT 9223372036854775807
        CHECK (max_price_ticks >= min_price_ticks),
    ADD COLUMN min_quantity_lots BIGINT NOT NULL DEFAULT 1
        CHECK (min_quantity_lots > 0),
    ADD COLUMN max_quantity_lots BIGINT NOT NULL DEFAULT 9223372036854775807
        CHECK (max_quantity_lots >= min_quantity_lots),
    ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1
        CHECK (config_version > 0);

-- +goose Down
ALTER TABLE markets
    DROP COLUMN IF EXISTS config_version,
    DROP COLUMN IF EXISTS max_quantity_lots,
    DROP COLUMN IF EXISTS min_quantity_lots,
    DROP COLUMN IF EXISTS max_price_ticks,
    DROP COLUMN IF EXISTS min_price_ticks,
    DROP COLUMN IF EXISTS lot_size_units,
    DROP COLUMN IF EXISTS tick_size_units,
    DROP COLUMN IF EXISTS quote_scale,
    DROP COLUMN IF EXISTS quantity_scale,
    DROP COLUMN IF EXISTS price_scale;
