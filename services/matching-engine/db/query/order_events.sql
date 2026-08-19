-- name: CreateOrderEvent :execrows
INSERT INTO order_events (
    event_id,
    command_id,
    command_sequence,
    event_index,
    order_id,
    user_id,
    ticker,
    event_type,
    reason,
    order_type,
    side,
    previous_price_ticks,
    price_ticks,
    previous_amount_lots,
    filled_amount_lots,
    canceled_amount_lots,
    remaining_amount_lots,
    market_config_version,
    occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
)
ON CONFLICT (event_id) DO NOTHING;

-- name: OrderEventPayloadMatches :one
SELECT (
    command_id = $2
    AND command_sequence = $3
    AND event_index = $4
    AND order_id = $5
    AND user_id = $6
    AND ticker = $7
    AND event_type = $8
    AND reason = $9
    AND order_type = $10
    AND side = $11
    AND previous_price_ticks = $12
    AND price_ticks = $13
    AND previous_amount_lots = $14
    AND filled_amount_lots = $15
    AND canceled_amount_lots = $16
    AND remaining_amount_lots = $17
    AND market_config_version = $18
    AND occurred_at = $19
) AS matches
FROM order_events
WHERE event_id = $1;
