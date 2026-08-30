#!/usr/bin/env bash
# 检查 internal/ 包覆盖率门禁（阈值见 docs/testing.md）。
set -euo pipefail
cd "$(dirname "$0")/.."

check_pkg() {
  local pkg=$1
  local min=$2
  local out
  out=$(mktemp)
  local line
  line=$(go test "./${pkg}/..." -count=1 -coverprofile="$out" 2>&1 | grep 'coverage:' || true)
  rm -f "$out"
  local pct
  pct=$(echo "$line" | sed -E 's/.*coverage: ([0-9.]+)% of statements.*/\1/')
  if [[ -z "$pct" ]]; then
    echo "WARN  ${pkg}: 无覆盖率数据"
    return 0
  fi
  if awk -v a="$pct" -v b="$min" 'BEGIN {exit !(a+0 >= b+0)}'; then
    echo "OK    ${pkg}: ${pct}% >= ${min}%"
  else
    echo "FAIL  ${pkg}: ${pct}% < ${min}%"
    return 1
  fi
}

FAIL=0
check_pkg internal/core 88 || FAIL=1
check_pkg internal/template 85 || FAIL=1
check_pkg internal/settings 74 || FAIL=1
check_pkg internal/license 70 || FAIL=1
check_pkg internal/app 30 || FAIL=1
check_pkg internal/excel 55 || FAIL=1

go test ./internal/... -count=1 -coverprofile=coverage.out -timeout 120s >/dev/null
TOTAL=$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/,"",$3); print $3}')
TOTAL_MIN=62
if awk -v a="$TOTAL" -v b="$TOTAL_MIN" 'BEGIN {exit !(a+0 >= b+0)}'; then
  echo "OK    total: ${TOTAL}% >= ${TOTAL_MIN}%"
else
  echo "FAIL  total: ${TOTAL}% < ${TOTAL_MIN}%"
  FAIL=1
fi

exit $FAIL
