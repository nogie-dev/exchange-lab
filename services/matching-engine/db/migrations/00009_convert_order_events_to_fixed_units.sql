-- +goose Up
ALTER TABLE order_events
    ADD COLUMN previous_price_ticks BIGINT,
    ADD COLUMN price_ticks BIGINT,
    ADD COLUMN previous_amount_lots BIGINT,
    ADD COLUMN filled_amount_lots BIGINT,
    ADD COLUMN canceled_amount_lots BIGINT,
    ADD COLUMN remaining_amount_lots BIGINT,
    ADD COLUMN market_config_version BIGINT NOT NULL DEFAULT 1
        CHECK (market_config_version > 0);

UPDATE order_events
SET previous_price_ticks = ROUND(previous_price::numeric * 100000000)::BIGINT,
    price_ticks = ROUND(price::numeric * 100000000)::BIGINT,
    previous_amount_lots = ROUND(previous_amount::numeric * 100000000)::BIGINT,
    filled_amount_lots = ROUND(filled_amount::numeric * 100000000)::BIGINT,
    canceled_amount_lots = ROUND(canceled_amount::numeric * 100000000)::BIGINT,
    remaining_amount_lots = ROUND(remaining_amount::numeric * 100000000)::BIGINT;

ALTER TABLE order_events
    ALTER COLUMN previous_price_ticks SET NOT NULL,
    ALTER COLUMN price_ticks SET NOT NULL,
    ALTER COLUMN previous_amount_lots SET NOT NULL,
    ALTER COLUMN filled_amount_lots SET NOT NULL,
    ALTER COLUMN canceled_amount_lots SET NOT NULL,
    ALTER COLUMN remaining_amount_lots SET NOT NULL,
    ADD CONSTRAINT order_events_previous_price_ticks_nonnegative CHECK (previous_price_ticks >= 0),
    ADD CONSTRAINT order_events_price_ticks_nonnegative CHECK (price_ticks >= 0),
    ADD CONSTRAINT order_events_previous_amount_lots_positive CHECK (previous_amount_lots > 0),
    ADD CONSTRAINT order_events_filled_amount_lots_nonnegative CHECK (filled_amount_lots >= 0),
    ADD CONSTRAINT order_events_canceled_amount_lots_nonnegative CHECK (canceled_amount_lots >= 0),
    ADD CONSTRAINT order_events_remaining_amount_lots_nonnegative CHECK (remaining_amount_lots >= 0),
    DROP COLUMN previous_price,
    DROP COLUMN price,
    DROP COLUMN previous_amount,
    DROP COLUMN filled_amount,
    DROP COLUMN canceled_amount,
    DROP COLUMN remaining_amount;

-- +goose Down
ALTER TABLE order_events
    ADD COLUMN previous_price DOUBLE PRECISION,
    ADD COLUMN price DOUBLE PRECISION,
    ADD COLUMN previous_amount DOUBLE PRECISION,
    ADD COLUMN filled_amount DOUBLE PRECISION,
    ADD COLUMN canceled_amount DOUBLE PRECISION,
    ADD COLUMN remaining_amount DOUBLE PRECISION;

UPDATE order_events
SET previous_price = previous_price_ticks / 100000000.0,
    price = price_ticks / 100000000.0,
    previous_amount = previous_amount_lots / 100000000.0,
    filled_amount = filled_amount_lots / 100000000.0,
    canceled_amount = canceled_amount_lots / 100000000.0,
    remaining_amount = remaining_amount_lots / 100000000.0;

ALTER TABLE order_events
    ALTER COLUMN previous_price SET NOT NULL,
    ALTER COLUMN price SET NOT NULL,
    ALTER COLUMN previous_amount SET NOT NULL,
    ALTER COLUMN filled_amount SET NOT NULL,
    ALTER COLUMN canceled_amount SET NOT NULL,
    ALTER COLUMN remaining_amount SET NOT NULL,
    DROP CONSTRAINT IF EXISTS order_events_previous_price_ticks_nonnegative,
    DROP CONSTRAINT IF EXISTS order_events_price_ticks_nonnegative,
    DROP CONSTRAINT IF EXISTS order_events_previous_amount_lots_positive,
    DROP CONSTRAINT IF EXISTS order_events_filled_amount_lots_nonnegative,
    DROP CONSTRAINT IF EXISTS order_events_canceled_amount_lots_nonnegative,
    DROP CONSTRAINT IF EXISTS order_events_remaining_amount_lots_nonnegative,
    DROP COLUMN IF EXISTS previous_price_ticks,
    DROP COLUMN IF EXISTS price_ticks,
    DROP COLUMN IF EXISTS previous_amount_lots,
    DROP COLUMN IF EXISTS filled_amount_lots,
    DROP COLUMN IF EXISTS canceled_amount_lots,
    DROP COLUMN IF EXISTS remaining_amount_lots,
    DROP COLUMN IF EXISTS market_config_version;
