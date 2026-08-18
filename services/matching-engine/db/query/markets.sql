-- name: CreateMarket :one
INSERT INTO markets (ticker)
VALUES ($1)
ON CONFLICT (ticker) DO NOTHING
RETURNING ticker, created_at;

-- name: GetMarket :one
SELECT ticker, created_at
FROM markets
WHERE ticker = $1;

-- name: ListMarkets :many
SELECT ticker, created_at
FROM markets
ORDER BY ticker;
