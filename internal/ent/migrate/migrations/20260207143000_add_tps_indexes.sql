-- Add index for TPS calculation queries
-- Migration for improving performance of TPS queries

-- Index for filtering completed executions with valid latency
-- Used in CTE to filter: WHERE status = 'completed' AND metrics_latency_ms > 0
-- This index is not definable in Ent schema (v0.14.5), so we use SQL migration

-- Create regular index for all supported database dialects (SQLite, PostgreSQL, MySQL, TiDB)
-- This is idempotent and works on all supported database dialects
-- Note: Partial indexes (WHERE clause) are not used here for cross-database compatibility
CREATE INDEX IF NOT EXISTS request_executions_status_latency_idx
ON request_executions(status, metrics_latency_ms);
