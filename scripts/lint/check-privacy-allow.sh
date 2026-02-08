#!/bin/bash
# 检查 Ent Privacy bypass 相关违规：
# 1) 禁止在 internal/authz 之外注入 DecisionContext(...Allow/Deny)
# 2) 限制 authz bypass API 调用位置（白名单 + gql/*.resolvers.go + *_internal.go）
# 3) 禁止 ctx = authz.WithBypassPrivacy(...) 的扩散写法

set -euo pipefail

echo "Checking privacy bypass governance rules..."

# 允许部分文件中的 authz bypass API 调用。
BYPASS_ALLOWLIST=(
  "internal/server/orchestrator/orchestrator.go"
  "internal/server/biz/auth.go"
  "internal/server/biz/quota.go"
  "internal/server/biz/permission_validator.go"
  "internal/server/biz/prompt.go"
  "internal/server/biz/system.go"
)

# 合法路径判断：注入 DecisionContext(...Allow/Deny)
# 仅允许：
# - internal/authz/*
is_allowed_allow_path() {
  local path="$1"

  [[ "$path" == internal/authz/* ]] && return 0

  return 1
}

# 合法路径判断：authz bypass API 调用
# 在 is_allowed_allow_path 基础上，额外允许：
# - *_internal.go
# - internal/server/gql/*.resolvers.go
# - internal/server/biz/system_*.go
# - *_test.go
# - internal/ent/migrate/datamigrate/*
is_allowed_bypass_api_path() {
  local path="$1"
  local allowed

  if is_allowed_allow_path "$path"; then
    return 0
  fi

  [[ "$path" == *_internal.go ]] && return 0
  [[ "$path" == internal/server/gql/*.resolvers.go ]] && return 0
  [[ "$path" == internal/server/biz/system_*.go ]] && return 0
  [[ "$path" == internal/ent/migrate/datamigrate/* ]] && return 0
  [[ "$path" == *_test.go ]] && return 0

  for allowed in "${BYPASS_ALLOWLIST[@]}"; do
    [[ "$path" == "$allowed" ]] && return 0
  done

  return 1
}

collect_violations() {
  local pattern="$1"
  local mode="$2"
  local raw
  local file

  raw=$(rg -n --no-heading --glob '*.go' "$pattern" . || true)
  if [[ -z "$raw" ]]; then
    return 0
  fi

  while IFS= read -r line; do
    [[ -z "$line" ]] && continue

    file="${line%%:*}"
    file="${file#./}"

    if [[ "$mode" == "allow" ]]; then
      if ! is_allowed_allow_path "$file"; then
        echo "$line"
      fi
      continue
    fi

    if ! is_allowed_bypass_api_path "$file"; then
      echo "$line"
    fi
  done <<< "$raw"
}

ALLOW_VIOLATIONS=$(collect_violations 'DecisionContext\s*\(.*\b(privacy\.)?Allow\b' 'allow')
DENY_VIOLATIONS=$(collect_violations 'DecisionContext\s*\(.*\b(privacy\.)?Deny\b' 'allow')
# Note: RunWithScopeDecision and WithScopeDecision are intentionally excluded from this check
# as they are scope-based authorization functions, not privacy bypass functions.
BYPASS_API_VIOLATIONS=$(collect_violations 'authz\.(WithBypassPrivacy|RunWithBypass|NewSystemBypassContext)\s*\(' 'bypass')
CTX_ASSIGN_VIOLATIONS=$(collect_violations 'ctx\s*[:=]\s*authz\.WithBypassPrivacy\s*\(' 'bypass')

has_error=0

if [[ -n "$ALLOW_VIOLATIONS" || -n "$DENY_VIOLATIONS" ]]; then
  echo "ERROR: Found illegal DecisionContext(...Allow/Deny) injection outside whitelist:"
  echo ""
  echo "$ALLOW_VIOLATIONS"
  echo "$DENY_VIOLATIONS"
  echo ""
  has_error=1
fi

if [[ -n "$BYPASS_API_VIOLATIONS" ]]; then
  echo "ERROR: Found authz bypass API usage outside allowed files (*_internal.go + migration allowlist):"
  echo ""
  echo "$BYPASS_API_VIOLATIONS"
  echo ""
  has_error=1
fi

if [[ -n "$CTX_ASSIGN_VIOLATIONS" ]]; then
  echo "ERROR: Found forbidden ctx assignment from authz.WithBypassPrivacy:"
  echo ""
  echo "$CTX_ASSIGN_VIOLATIONS"
  echo ""
  has_error=1
fi

if [[ "$has_error" -eq 1 ]]; then
  echo "See docs/zh/development/ent-privacy-governance-plan.md for governance rules."
  echo "Note: migration allowlist is temporary and should be reduced to zero over time."
  exit 1
fi

echo "OK: Privacy bypass governance checks passed."
