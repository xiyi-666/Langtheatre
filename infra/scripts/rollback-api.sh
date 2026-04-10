#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${TARGET_HOST:-}" ]]; then
  echo "TARGET_HOST is required"
  exit 1
fi

if [[ -z "${TARGET_PATH:-}" ]]; then
  TARGET_PATH="/srv/linguaquest"
fi

ssh "${TARGET_HOST}" "cp ${TARGET_PATH}/linguaquest-api.prev ${TARGET_PATH}/linguaquest-api && systemctl restart linguaquest-api"
echo "Rollback done."
