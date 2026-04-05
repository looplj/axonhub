-- Rename node_id to server_fingerprint in requests table
ALTER TABLE requests CHANGE COLUMN node_id server_fingerprint VARCHAR(255) NULL;

-- Rename index for requests table
ALTER TABLE requests RENAME INDEX requests_by_node_id_status TO requests_by_server_fingerprint_status;

-- Rename node_id to server_fingerprint in request_executions table
ALTER TABLE request_executions CHANGE COLUMN node_id server_fingerprint VARCHAR(255) NULL;

-- Rename index for request_executions table
ALTER TABLE request_executions RENAME INDEX request_executions_by_node_id_status TO request_executions_by_server_fingerprint_status;
