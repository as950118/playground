#!/usr/bin/env bash
# Measure range-query response time against any Prometheus-compatible HTTP API
# (works against Prometheus itself or Thanos Query, since Thanos Query implements
# the same /api/v1 endpoints).
#
# Usage: query_bench.sh <base-url> ["promql query"] [range-seconds]
# Example: ./common/scripts/query_bench.sh http://localhost:9090 'up' 3600
set -euo pipefail

BASE_URL="${1:?base url required, e.g. http://localhost:9090 or http://localhost:10904}"
QUERY="${2:-up}"
RANGE_SECONDS="${3:-3600}"

END=$(date +%s)
START=$((END - RANGE_SECONDS))

echo ">>> range query against ${BASE_URL} (query=${QUERY}, range=${RANGE_SECONDS}s)"
curl -s -o /tmp/query_bench_response.json -w \
  "http_code=%{http_code}  time_total=%{time_total}s  size=%{size_download} bytes\n" \
  --data-urlencode "query=${QUERY}" \
  --data-urlencode "start=${START}" \
  --data-urlencode "end=${END}" \
  --data-urlencode "step=15s" \
  "${BASE_URL}/api/v1/query_range"

echo "response saved to /tmp/query_bench_response.json"
