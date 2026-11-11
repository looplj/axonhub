#!/bin/sh
# Upgrade every product Go version pin in one pass.
# Usage: scripts/bump-go-version.sh 1.27.5

set -eu

usage() {
  printf 'usage: %s <go-version>\n' "$0" >&2
  printf 'example: %s 1.27.5\n' "$0" >&2
  exit 2
}

if [ "${1:-}" = "" ] || [ "${2:-}" != "" ]; then
  usage
fi

version=$1
if ! printf '%s\n' "$version" | awk -F. 'NF == 3 && $1 ~ /^[0-9]+$/ && $2 ~ /^[0-9]+$/ && $3 ~ /^[0-9]+$/ { ok = 1 } END { exit !ok }'; then
  printf 'go version must look like 1.27.5, got %s\n' "$version" >&2
  exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

current=$(awk '/^go / { print $2; exit }' go.mod)
if [ -z "$current" ]; then
  printf 'could not read the go version from go.mod\n' >&2
  exit 1
fi

changed=0

write_if_changed() {
  file=$1
  tmp=$2
  if cmp -s "$file" "$tmp"; then
    rm -f "$tmp"
    return 0
  fi
  mv "$tmp" "$file"
  printf 'updated %s\n' "$file"
  changed=1
}

fail_rewrite() {
  file=$1
  tmp=$2
  rm -f "$tmp"
  printf 'failed to update %s\n' "$file" >&2
  exit 1
}

update_go_mod() {
  file=$1
  if [ ! -f "$file" ]; then
    printf 'missing %s\n' "$file" >&2
    exit 1
  fi
  tmp=$(mktemp)
  if ! awk -v version="$version" '
    BEGIN { found = 0 }
    /^go / && !found {
      print "go " version
      found = 1
      next
    }
    { print }
    END { if (!found) exit 1 }
  ' "$file" >"$tmp"; then
    fail_rewrite "$file" "$tmp"
  fi
  write_if_changed "$file" "$tmp"
}

update_dockerfile() {
  file=Dockerfile
  tmp=$(mktemp)
  if ! awk -v version="$version" '
    BEGIN { found = 0 }
    /^FROM golang:/ {
      if (!sub(/golang:[0-9]+\.[0-9]+\.[0-9]+/, "golang:" version)) exit 1
      found = 1
    }
    { print }
    END { if (!found) exit 1 }
  ' "$file" >"$tmp"; then
    fail_rewrite "$file" "$tmp"
  fi
  write_if_changed "$file" "$tmp"
}

update_prose() {
  file=$1
  if [ ! -f "$file" ]; then
    printf 'missing %s\n' "$file" >&2
    exit 1
  fi
  tmp=$(mktemp)
  if ! awk -v old="$current" -v new="$version" '
    BEGIN { found = 0 }
    {
      line = $0
      out = ""
      while (match(line, /[0-9]+\.[0-9]+\.[0-9]+/)) {
        token = substr(line, RSTART, RLENGTH)
        out = out substr(line, 1, RSTART - 1)
        if (token == old) {
          out = out new
          found = 1
        } else {
          out = out token
        }
        line = substr(line, RSTART + RLENGTH)
      }
      print out line
    }
    END { if (old != new && !found) exit 1 }
  ' "$file" >"$tmp"; then
    fail_rewrite "$file" "$tmp"
  fi
  write_if_changed "$file" "$tmp"
}

update_workflow() {
  file=$1
  tmp=$(mktemp)
  quoted="'$version'"
  awk -v quoted="$quoted" '
    /^[[:space:]]*#/ { print; next }
    /^[[:space:]]*go-version-file:/ { print; next }
    /^[[:space:]]*go-version:/ {
      sub(/go-version:.*/, "go-version: " quoted)
      print
      next
    }
    { print }
  ' "$file" >"$tmp"
  write_if_changed "$file" "$tmp"
}

for module in \
  go.mod \
  llm/go.mod \
  cmd/schema/go.mod \
  cmd/pack-plugin/go.mod \
  cmd/axonhub-desktop/go.mod
do
  update_go_mod "$module"
done

update_dockerfile

for prose in \
  AGENTS.md \
  docs/zh/development/development.md \
  docs/en/development/development.md
do
  update_prose "$prose"
done

for workflow in .github/workflows/*.yml .github/workflows/*.yaml; do
  if [ -f "$workflow" ]; then
    update_workflow "$workflow"
  fi
done

if [ "$changed" -eq 0 ]; then
  printf 'already at %s\n' "$version"
else
  printf 'go version is now %s\n' "$version"
fi
