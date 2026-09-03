#!/usr/bin/env bash
# Stop one component of a stack and confirm whether the query API still answers.
# This is the quick HA / failure-tolerance check used to compare the 3 stacks:
# stopping a Prometheus shard (02) or a receive replica (03) should NOT break
# the global query view if Thanos is doing its job; stopping the single
# Prometheus in 01 SHOULD break it (that's the point of the baseline).
#
# Usage: kill_and_check.sh <stack-dir> <service-to-stop> <query-base-url> ["promql query"]
# Example: ./common/scripts/kill_and_check.sh ./02-thanos-sidecar-3x prometheus-shard-1 http://localhost:10904 up
set -euo pipefail

STACK_DIR="${1:?path to a stack directory required}"
SERVICE="${2:?service name to stop required, see docker-compose.yml}"
BASE_URL="${3:?query api base url required}"
QUERY="${4:-up}"
COMPOSE_FILE="${STACK_DIR}/docker-compose.yml"

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose -f "${COMPOSE_FILE}")
else
  COMPOSE=(docker-compose -f "${COMPOSE_FILE}")
fi

echo ">>> stopping '${SERVICE}' in ${STACK_DIR}"
"${COMPOSE[@]}" stop "${SERVICE}"

sleep 3
echo ">>> querying ${BASE_URL}/api/v1/query?query=${QUERY} after stopping ${SERVICE}"
curl -s -w "\nhttp_code=%{http_code}\n" \
  --data-urlencode "query=${QUERY}" \
  "${BASE_URL}/api/v1/query" || echo "(no response -- query endpoint is unreachable, e.g. the single-Prometheus baseline)"

echo
echo ">>> restarting '${SERVICE}'"
"${COMPOSE[@]}" start "${SERVICE}"
