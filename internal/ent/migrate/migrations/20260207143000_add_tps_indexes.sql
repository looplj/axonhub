-- Add partial index for TPS calculation queries
-- Migration for improving performance of TPS queries

-- Partial index for filtering completed executions with valid latency
-- Used in CTE to filter: WHERE status = 'completed' AND metrics_latency_ms > 0
-- This index is not definable in Ent schema (v0.14.5), so we use SQL migration

-- Try PostgreSQL-specific partial index first (works on PostgreSQL 9.5+)
DO $$
BEGIN
    CREATE INDEX IF NOT EXISTS request_executions_status_latency_idx
    ON request_executions(status, metrics_latency_ms)
    WHERE status = 'completed' AND metrics_latency_ms > 0;
EXCEPTION
    WHEN OTHERS THEN NULL; -- Not PostgreSQL, will create regular index below
END $$;

-- Create regular index for SQLite, MySQL, TiDB, and as fallback for PostgreSQL
-- This is idempotent and works on all supported database dialects
CREATE INDEX IF NOT EXISTS request_executions_status_latency_regular_idx
ON request_executions(status, metrics_latency_ms);