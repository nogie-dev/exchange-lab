-- +goose Up
CREATE TABLE order_events (
    order_event_id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    command_id TEXT NOT NULL,
    command_sequence BIGINT NOT NULL CHECK (command_sequence > 0),
    event_index INTEGER NOT NULL CHECK (event_index >= 0),
    order_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    ticker TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'ORDER_RESTING',
        'ORDER_PARTIALLY_FILLED',
        'ORDER_FILLED',
        'ORDER_AMENDED',
        'ORDER_CANCELED',
        'ORDER_REMAINDER_CANCELED'
    )),
    reason TEXT NOT NULL CHECK (reason <> ''),
    order_type TEXT NOT NULL CHECK (order_type IN ('LIMIT', 'MARKET')),
    side TEXT NOT NULL CHECK (side IN ('BID', 'ASK')),
    previous_price DOUBLE PRECISION NOT NULL CHECK (previous_price >= 0),
    price DOUBLE PRECISION NOT NULL CHECK (price >= 0),
    previous_amount DOUBLE PRECISION NOT NULL CHECK (previous_amount > 0),
    filled_amount DOUBLE PRECISION NOT NULL CHECK (filled_amount >= 0),
    canceled_amount DOUBLE PRECISION NOT NULL CHECK (canceled_amount >= 0),
    remaining_amount DOUBLE PRECISION NOT NULL CHECK (remaining_amount >= 0),
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ticker, command_sequence, event_index)
);

CREATE INDEX order_events_user_occurred_at_idx
    ON order_events (user_id, occurred_at DESC, order_event_id DESC);
CREATE INDEX order_events_order_occurred_at_idx
    ON order_events (order_id, occurred_at, order_event_id);

-- +goose Down
DROP TABLE IF EXISTS order_events;
