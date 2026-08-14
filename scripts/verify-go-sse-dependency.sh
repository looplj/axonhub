#!/bin/sh
set -eu

module="github.com/tmaxmax/go-sse"
expected_path="github.com/wtj-0527/go-sse"
expected_version="v0.0.0-20260811060543-0bb36b8ea0cd"
binary="${1:-}"
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

resolved_dependency() {
  module_dir=$1
  (
    cd "$module_dir"
    go list -m -f '{{if .Replace}}{{.Replace.Path}}@{{.Replace.Version}}{{else}}{{.Path}}@{{.Version}}{{end}}' "$module"
  )
}

expected="$expected_path@$expected_version"
for module_dir in "$repo_root" "$repo_root/llm" "$repo_root/cmd/schema"; do
  actual=$(resolved_dependency "$module_dir")
  if [ "$actual" != "$expected" ]; then
    printf 'go-sse dependency mismatch in %s: got %s, want %s\n' "$module_dir" "$actual" "$expected" >&2
    exit 1
  fi
  printf 'verified module dependency: %s -> %s\n' "$module_dir" "$actual"
done

if [ -n "$binary" ]; then
  case "$binary" in
    /*) ;;
    *) binary="$repo_root/$binary" ;;
  esac
  if [ ! -x "$binary" ]; then
    printf 'production binary is missing or not executable: %s\n' "$binary" >&2
    exit 1
  fi

  build_info=$(go version -m "$binary")
  tab=$(printf '\t')
  if ! printf '%s\n' "$build_info" | awk -F "$tab" -v module="$module" -v version="v0.11.0" -v replacement_path="$expected_path" -v replacement_version="$expected_version" '
    $2 == "dep" && $3 == module && $4 == version {
      if (getline next_line <= 0) {
        exit 1
      }
      field_count = split(next_line, fields, FS)
      if (field_count < 4 || fields[2] != "=>" || fields[3] != replacement_path || fields[4] != replacement_version) {
        exit 1
      }
      found = 1
      exit 0
    }
    END { if (!found) exit 1 }
  '; then
    printf 'production binary has wrong or unassociated go-sse replacement; want %s directly after %s@v0.11.0\n%s\n' "$expected" "$module" "$build_info" >&2
    exit 1
  fi
  printf 'verified production binary dependency: %s -> %s\n' "$binary" "$expected"
fi
