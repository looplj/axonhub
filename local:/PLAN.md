# Refactor: Rename node_id to server_fingerprint

## Overview
Based on code review feedback, rename `node_id`/`nodeID` to `server_fingerprint` to avoid confusion with GraphQL node IDs, and add a hash for better uniqueness.

## Changes Required

### 1. Schema Updates
- Update `internal/ent/schema/request.go`: Rename field from `node_id` to `server_fingerprint`
- Update `internal/ent/schema/request_execution.go`: Rename field from `node_id` to `server_fingerprint`

### 2. Service Implementation
- Update `internal/server/biz/request.go`:
  - Rename `getNodeID()` to `getServerFingerprint()`
  - Add hash (SHA256) for better uniqueness
  - Rename `nodeID` field to `serverFingerprint`
  - Update all references throughout the file

### 3. Regenerate Ent Code
- Run `go generate ./internal/ent` to regenerate all ent code with the new field name

### 4. Database Migration
- Create a new migration to rename the column in both tables:
  - `requests.node_id` → `requests.server_fingerprint`
  - `request_executions.node_id` → `request_executions.server_fingerprint`
- Rename indexes:
  - `requests_by_node_id_status` → `requests_by_server_fingerprint_status`
  - `request_executions_by_node_id_status` → `request_executions_by_server_fingerprint_status`

## Implementation Details

### Hash Implementation
```go
func getServerFingerprint() string {
    hostname, err := os.Hostname()
    if err != nil || hostname == "" {
        hostname = fmt.Sprintf("node-%d", os.Getpid())
    }
    
    // Combine hostname and PID for uniqueness
    data := fmt.Sprintf("%s-%d", hostname, os.Getpid())
    
    // Add hash for cryptographic uniqueness
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%s-%x", data, hash[:8])
}
```

This provides:
- Better uniqueness through cryptographic hash
- Human-readable prefix for debugging
- Avoids confusion with GraphQL node IDs

## Testing
- Verify that the server fingerprint is unique across restarts
- Test multi-node cleanup scenarios
- Verify GraphQL queries still work correctly
- Run existing tests

## Benefits
1. Clear naming that distinguishes from GraphQL node IDs
2. Cryptographic hash adds uniqueness and security
3. Better represents the purpose (server instance identification)
