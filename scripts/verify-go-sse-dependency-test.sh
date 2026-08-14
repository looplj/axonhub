#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
verifier="$repo_root/scripts/verify-go-sse-dependency.sh"
expected_path="github.com/wtj-0527/go-sse"
expected_version="v0.0.0-20260811060543-0bb36b8ea0cd"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin"

cat >"$tmp/bin/go" <<'EOF'
#!/bin/sh
set -eu
if [ "${1:-}" = list ] && [ "${2:-}" = -m ]; then
  printf '%s\n' 'github.com/wtj-0527/go-sse@v0.0.0-20260811060543-0bb36b8ea0cd'
elif [ "${1:-}" = version ] && [ "${2:-}" = -m ]; then
  cat "$GO_VERSION_M_FIXTURE"
else
  printf 'unexpected fake go invocation: %s\n' "$*" >&2
  exit 2
fi
EOF
chmod +x "$tmp/bin/go"

{
  printf '/tmp/axonhub: go1.25.0\n'
  printf '\tdep\tgithub.com/tmaxmax/go-sse\tv0.11.0\n'
  printf '\t=>\t%s\t%s\th1:valid-checksum\n' "$expected_path" "$expected_version"
} >"$tmp/valid.txt"

{
  printf '/tmp/axonhub: go1.25.0\n'
  printf '\tdep\tgithub.com/tmaxmax/go-sse\tv0.11.0\n'
  printf '\tdep\texample.com/unrelated\tv1.0.0\n'
  printf '\t=>\t%s\t%s\th1:spoof-checksum\n' "$expected_path" "$expected_version"
} >"$tmp/spoof.txt"

binary="$tmp/axonhub"
printf '#!/bin/sh\nexit 0\n' >"$binary"
chmod +x "$binary"

PATH="$tmp/bin:$PATH" GO_VERSION_M_FIXTURE="$tmp/valid.txt" "$verifier" "$binary" >/dev/null

if PATH="$tmp/bin:$PATH" GO_VERSION_M_FIXTURE="$tmp/spoof.txt" "$verifier" "$binary" >/dev/null 2>&1; then
  echo 'verifier accepted an unrelated replacement record' >&2
  exit 1
fi

echo 'verified production dependency parser rejects unrelated replacement records'
