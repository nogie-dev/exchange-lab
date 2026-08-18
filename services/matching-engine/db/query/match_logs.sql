-- name: CreateMatchLog :execrows
INSERT INTO match_logs (
    execution_id,
    ticker,
    price,
    amount,
    quote_amount,
    maker_order_id,
    taker_order_id,
    maker_user_id,
    taker_user_id,
    maker_side,
    taker_side,
    matched_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (execution_id) DO NOTHING;

-- name: MatchLogPayloadMatches :one
SELECT (
    ticker = $2
    AND price = $3
    AND amount = $4
    AND quote_amount = $5
    AND maker_order_id = $6
    AND taker_order_id = $7
    AND maker_user_id = $8
    AND taker_user_id = $9
    AND maker_side = $10
    AND taker_side = $11
    AND matched_at = $12
) AS matches
FROM match_logs
WHERE execution_id = $1;
