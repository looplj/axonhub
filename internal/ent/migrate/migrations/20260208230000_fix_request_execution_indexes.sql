-- Remove redundant index (request_id is covered by composite index)
DROP INDEX IF EXISTS request_executions_by_request_id;

-- Rename index to follow naming convention
ALTER TABLE request_executions
RENAME INDEX request_executions_request_status_created_idx
TO request_executions_by_request_id_status_created_at;