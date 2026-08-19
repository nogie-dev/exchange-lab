-- name: CreateMarket :one
INSERT INTO markets (ticker)
VALUES ($1)
ON CONFLICT (ticker) DO NOTHING
RETURNING ticker, created_at, price_scale, quantity_scale, quote_scale,
    tick_size_units, lot_size_units, min_price_ticks, max_price_ticks,
    min_quantity_lots, max_quantity_lots, config_version;

-- name: GetMarket :one
SELECT ticker, created_at, price_scale, quantity_scale, quote_scale,
    tick_size_units, lot_size_units, min_price_ticks, max_price_ticks,
    min_quantity_lots, max_quantity_lots, config_version
FROM markets
WHERE ticker = $1;

-- name: ListMarkets :many
SELECT ticker, created_at, price_scale, quantity_scale, quote_scale,
    tick_size_units, lot_size_units, min_price_ticks, max_price_ticks,
    min_quantity_lots, max_quantity_lots, config_version
FROM markets
ORDER BY ticker;
