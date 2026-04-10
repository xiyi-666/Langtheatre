#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${TARGET_HOST:-}" ]]; then
  echo "TARGET_HOST is required"
  exit 1
fi

if [[ -z "${TARGET_PATH:-}" ]]; then
  TARGET_PATH="/srv/linguaquest"
fi

echo "Building API binary..."
cd "$(dirname "$0")/../../apps/server"
go build -o linguaquest-api ./cmd/server

echo "Uploading to ${TARGET_HOST}:${TARGET_PATH} ..."
scp linguaquest-api "${TARGET_HOST}:${TARGET_PATH}/linguaquest-api.new"

echo "Switching binary on remote..."
ssh "${TARGET_HOST}" "mv ${TARGET_PATH}/linguaquest-api.new ${TARGET_PATH}/linguaquest-api && systemctl restart linguaquest-api"

echo "Deployment done."
