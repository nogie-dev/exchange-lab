-- name: CreateMatchLog :exec
INSERT INTO match_logs (
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
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);
