-- +goose Up
ALTER TABLE match_logs
    ADD COLUMN price_ticks BIGINT,
    ADD COLUMN amount_lots BIGINT,
    ADD COLUMN quote_amount_atoms BIGINT,
    ADD COLUMN market_config_version BIGINT NOT NULL DEFAULT 1
        CHECK (market_config_version > 0);

-- The first market configuration uses eight decimal places for price,
-- quantity, and quote values. Existing local rows are converted before the
-- legacy floating-point columns are removed.
UPDATE match_logs
SET price_ticks = ROUND(price::numeric * 100000000)::BIGINT,
    amount_lots = ROUND(amount::numeric * 100000000)::BIGINT,
    quote_amount_atoms = ROUND(quote_amount::numeric * 100000000)::BIGINT;

ALTER TABLE match_logs
    ALTER COLUMN price_ticks SET NOT NULL,
    ALTER COLUMN amount_lots SET NOT NULL,
    ALTER COLUMN quote_amount_atoms SET NOT NULL,
    ADD CONSTRAINT match_logs_price_ticks_positive CHECK (price_ticks > 0),
    ADD CONSTRAINT match_logs_amount_lots_positive CHECK (amount_lots > 0),
    ADD CONSTRAINT match_logs_quote_amount_atoms_nonnegative CHECK (quote_amount_atoms >= 0),
    DROP COLUMN price,
    DROP COLUMN amount,
    DROP COLUMN quote_amount;

-- +goose Down
ALTER TABLE match_logs
    ADD COLUMN price DOUBLE PRECISION,
    ADD COLUMN amount DOUBLE PRECISION,
    ADD COLUMN quote_amount DOUBLE PRECISION;

UPDATE match_logs
SET price = price_ticks / 100000000.0,
    amount = amount_lots / 100000000.0,
    quote_amount = quote_amount_atoms / 100000000.0;

ALTER TABLE match_logs
    ALTER COLUMN price SET NOT NULL,
    ALTER COLUMN amount SET NOT NULL,
    ALTER COLUMN quote_amount SET NOT NULL,
    DROP CONSTRAINT IF EXISTS match_logs_price_ticks_positive,
    DROP CONSTRAINT IF EXISTS match_logs_amount_lots_positive,
    DROP CONSTRAINT IF EXISTS match_logs_quote_amount_atoms_nonnegative,
    DROP COLUMN IF EXISTS price_ticks,
    DROP COLUMN IF EXISTS amount_lots,
    DROP COLUMN IF EXISTS quote_amount_atoms,
    DROP COLUMN IF EXISTS market_config_version;
