#!/usr/bin/env bash
# Report on-disk size of TSDB data volumes for every running container in a stack,
# plus the MinIO object-storage bucket size when the stack has one (02/03).
#
# Usage: disk_usage.sh <path-to-stack-dir>
# Example: ./common/scripts/disk_usage.sh ./02-thanos-sidecar-3x
set -euo pipefail

STACK_DIR="${1:?path to a stack directory required, e.g. ./01-single-prometheus}"
COMPOSE_FILE="${STACK_DIR}/docker-compose.yml"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "no docker-compose.yml found at ${COMPOSE_FILE}" >&2
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose -f "${COMPOSE_FILE}")
else
  COMPOSE=(docker-compose -f "${COMPOSE_FILE}")
fi

echo "== container data directory sizes for ${STACK_DIR} =="
for cid in $("${COMPOSE[@]}" ps -q); do
  name=$(docker inspect -f '{{.Name}}' "${cid}" | sed 's#^/##')
  size=$(docker exec "${cid}" sh -c \
    'du -sh /prometheus 2>/dev/null || du -sh /prometheus-agent 2>/dev/null || du -sh /data 2>/dev/null || echo "n/a"' \
    2>/dev/null || echo "n/a")
  printf '%-30s %s\n' "${name}" "${size}"
done

echo
echo "== MinIO bucket usage (if this stack has MinIO) =="
minio_cid=$("${COMPOSE[@]}" ps -q minio 2>/dev/null || true)
if [[ -n "${minio_cid}" ]]; then
  docker exec "${minio_cid}" sh -c 'du -sh /data 2>/dev/null || echo "n/a"'
else
  echo "no minio service in this stack"
fi
