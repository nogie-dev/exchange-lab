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
    previous_price,
    price,
    previous_amount,
    filled_amount,
    canceled_amount,
    remaining_amount,
    occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14, $15, $16, $17, $18
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
    AND previous_price = $12
    AND price = $13
    AND previous_amount = $14
    AND filled_amount = $15
    AND canceled_amount = $16
    AND remaining_amount = $17
    AND occurred_at = $18
) AS matches
FROM order_events
WHERE event_id = $1;
