

---

### Task 5: Write Integration Tests

**What to do**:
- Create integration tests in `integration_test/openai/models/` directory
- Test the actual `/v1/models` endpoint with extended parameter
- Verify response structure and field types

**Must NOT do**:
- Skip backward compatibility test
- Skip extended format test
- Test against production endpoints

**Recommended Agent Profile**:
- **Category**: `quick`
- **Skills**: []
- Reason: Integration testing following existing patterns

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 3 (with Task 4)
- **Blocks**: Task 6
- **Blocked By**: Task 3

**References**:
- `integration_test/openai/chat/` - Existing OpenAI integration tests
- `integration_test/setup_test.go` - Test setup and helpers

**Acceptance Criteria**:
- [x] Integration test file created: `integration_test/openai/models/models_test.go`
- [x] Test backward compatibility: `GET /v1/models` returns original format
- [x] Test extended format: `GET /v1/models?include=all` returns extended format
- [x] Test field types: all fields have correct JSON types
- [x] Tests run with: `go test ./integration_test/openai/models/... -v`

**Verification Commands**:
```bash
# Run integration tests
go test ./integration_test/openai/models/... -v
# Assert: All tests pass (PASS)

# Manual verification
curl -s "http://localhost:8090/v1/models?extended=true" | jq '.data[0]'
# Assert: Response contains all new fields
```

**Commit**: YES (group with Task 4)
- Message: `test(integration): add OpenAI models endpoint tests`
- Files: `integration_test/openai/models/models_test.go`
- Pre-commit: `go test ./integration_test/openai/models/...`

---

### Task 6: Final Verification and Commit

**What to do**:
- Run full test suite to ensure no regressions
- Verify all commits are clean and follow conventional commits format
- Final code review check
- Push feature branch

**Must NOT do**:
- Skip test suite
- Skip linting
- Merge to main (just push feature branch)

**Recommended Agent Profile**:
- **Category**: `quick`
- **Skills**: `git-master`
- Reason: Final verification and git operations

**Parallelization**:
- **Can Run In Parallel**: NO
- **Parallel Group**: Sequential
- **Blocks**: None
- **Blocked By**: Task 4, Task 5

**Acceptance Criteria**:
- [x] All unit tests pass: `go test ./internal/server/api/... -v`
- [x] All integration tests pass: `go test ./integration_test/openai/models/... -v`
- [x] Backend builds successfully: `make build-backend`
- [x] Lint passes: `golangci-lint run ./internal/server/api/...`
- [x] Feature branch pushed: `git push origin feature/enhance-models-endpoint`
- [x] All commits follow conventional commits format

**Verification Commands**:
```bash
# Full verification
make build-backend
go test ./internal/server/api/... -v
go test ./integration_test/openai/models/... -v
golangci-lint run ./internal/server/api/...

# Git verification
git log --oneline feature/enhance-models-endpoint
# Assert: All commits follow "type(scope): description" format
git branch -r | grep feature/enhance-models-endpoint
# Assert: Branch exists on remote
```

**Commit**: NO (just verification)

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `feat(api): extend OpenAIModel struct with metadata fields` | `internal/server/api/openai.go` | `go build ./internal/server/api/...` |
| 2 | `feat(api): add model transformation helpers with nil safety` | `internal/server/api/openai.go`, `internal/server/api/openai_model_test.go` | `go test ./internal/server/api/...` |
| 3 | `feat(api): enhance ListModels with extended metadata support` | `internal/server/api/openai.go` | `go build ./internal/server/api/... && go test ./internal/server/api/...` |
| 4 & 5 | `test(api): add unit and integration tests for model endpoint` | `internal/server/api/openai_model_test.go`, `integration_test/openai/models/models_test.go` | `go test ./internal/server/api/... ./integration_test/openai/models/...` |

---

## Success Criteria

### Verification Commands
```bash
# Test backward compatibility
curl -s http://localhost:8090/v1/models | jq '.data[0] | keys'
# Expected: ["created", "id", "object", "owned_by"]

# Test extended format
curl -s "http://localhost:8090/v1/models?extended=true" | jq '.data[0] | keys'
# Expected: ["capabilities", "context_length", "created", "description", "icon", "id", "max_output_tokens", "name", "object", "owned_by", "pricing", "type"]

# Test field types
curl -s "http://localhost:8090/v1/models?extended=true" | jq '.data[0].capabilities.vision'
# Expected: boolean or null

# Run all tests
go test ./internal/server/api/... ./integration_test/openai/models/...
# Expected: PASS
```

### Final Checklist
- [x] All TODOs completed
- [x] All tests passing
- [x] Backward compatibility verified
- [x] Feature branch pushed to origin
- [x] No linting errors
- [x] Follows Go coding conventions from AGENTS.md

---

## Guardrails Summary

**CRITICAL CONSTRAINTS**:
1. **Backward Compatibility**: Existing response format must remain unchanged when `?extended` is not provided
2. **Nil Safety**: All nested struct access must have nil checks to prevent panics
3. **No Breaking Changes**: Cannot modify existing `OpenAIModel` fields or JSON tags
4. **Scope Boundaries**: OpenAI endpoint only - no changes to Anthropic/Gemini
5. **No Schema Changes**: Use existing database fields only

**AI SLOP PATTERNS TO AVOID**:
- Do not add unrelated features or refactor existing code
- Do not change error handling patterns
- Do not add premature optimizations (caching, pagination)
- Do not skip tests for "simple" changes
- Do not use mock frameworks unnecessarily
