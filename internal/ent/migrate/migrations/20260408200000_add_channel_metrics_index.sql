CREATE INDEX IF NOT EXISTS request_executions_status_created_idx
ON request_executions(status, created_at);

CREATE INDEX IF NOT EXISTS request_executions_metrics_load_idx
ON request_executions(created_at, status, channel_id, model_id);
