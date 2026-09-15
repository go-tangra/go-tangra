#!/usr/bin/env bash
# SC-006: Freya channel vs a plaintext gRPC server running identical non-security
# middleware. Defaults: sequential p50 ratio <= 1.15, 8-way throughput ratio <= 1.25.
set -euo pipefail
f="${1:-.artifacts/bench.txt}"
MAX_CALL="${BENCH_MAX_RATIO:-1.15}"
MAX_TP="${BENCH_MAX_THROUGHPUT_RATIO:-1.25}"
ratio() {
  local plain mtls
  plain=$(awk -v n="$1/plaintext" 'index($1, n)==1 {print $3}' "$f" | head -1)
  mtls=$(awk -v n="$1/mtls" 'index($1, n)==1 {print $3}' "$f" | head -1)
  [[ -n "$plain" && -n "$mtls" ]] || { echo "bench-gate: missing $1 results in $f" >&2; exit 1; }
  awk -v p="$plain" -v m="$mtls" -v max="$2" -v name="$1" 'BEGIN {
    r=m/p; printf "bench-gate: %s mtls/plaintext ns/op ratio = %.3f (max %s)\n", name, r, max
    if (r > max+0) { printf "bench-gate: %s overhead exceeds the threshold\n", name > "/dev/stderr"; exit 1 } }'
}
ratio BenchmarkCall "$MAX_CALL"
ratio BenchmarkThroughput "$MAX_TP"
