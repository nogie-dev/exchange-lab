-- name: CreateMatchLog :execrows
INSERT INTO match_logs (
    execution_id,
    ticker,
    price_ticks,
    amount_lots,
    quote_amount_atoms,
    market_config_version,
    maker_order_id,
    taker_order_id,
    maker_user_id,
    taker_user_id,
    maker_side,
    taker_side,
    matched_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (execution_id) DO NOTHING;

-- name: MatchLogPayloadMatches :one
SELECT (
    ticker = $2
    AND price_ticks = $3
    AND amount_lots = $4
    AND quote_amount_atoms = $5
    AND market_config_version = $6
    AND maker_order_id = $7
    AND taker_order_id = $8
    AND maker_user_id = $9
    AND taker_user_id = $10
    AND maker_side = $11
    AND taker_side = $12
    AND matched_at = $13
) AS matches
FROM match_logs
WHERE execution_id = $1;
