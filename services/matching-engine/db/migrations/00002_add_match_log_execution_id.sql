-- +goose Up
ALTER TABLE match_logs ADD COLUMN execution_id TEXT;

UPDATE match_logs
SET execution_id = 'legacy-' || match_log_id::TEXT;

ALTER TABLE match_logs ALTER COLUMN execution_id SET NOT NULL;
ALTER TABLE match_logs
    ADD CONSTRAINT match_logs_execution_id_key UNIQUE (execution_id);

-- +goose Down
ALTER TABLE match_logs DROP COLUMN execution_id;
