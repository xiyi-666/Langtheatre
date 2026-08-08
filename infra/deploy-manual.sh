#!/usr/bin/env bash

set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/linguaquest}"
ENV_FILE="${ENV_FILE:-$DEPLOY_DIR/.env.production}"
COMPOSE_FILE="${COMPOSE_FILE:-$DEPLOY_DIR/docker-compose.deploy.yml}"
NETWORK_NAME="${NETWORK_NAME:-linguaquest-network}"

require_file() {
  if [ ! -f "$1" ]; then
    echo "Required deployment file does not exist: $1" >&2
    exit 1
  fi
}

attach_existing_container() {
  container_name="$1"
  required="$2"

  if ! docker container inspect "$container_name" >/dev/null 2>&1; then
    if [ "$required" = "true" ]; then
      echo "Required container $container_name does not exist." >&2
      exit 1
    fi
    echo "Optional container $container_name does not exist; skipping."
    return 0
  fi

  if [ "$(docker inspect --format '{{.State.Running}}' "$container_name")" != "true" ]; then
    if [ "$required" = "true" ]; then
      echo "Required container $container_name is not running." >&2
      exit 1
    fi
    echo "Optional container $container_name is not running; skipping."
    return 0
  fi

  if docker inspect --format '{{json .NetworkSettings.Networks}}' "$container_name" | grep -q "\"$NETWORK_NAME\""; then
    echo "$container_name is already connected to $NETWORK_NAME."
    return 0
  fi

  docker network connect "$NETWORK_NAME" "$container_name"
  echo "Connected $container_name to $NETWORK_NAME."
}

require_file "$ENV_FILE"
require_file "$COMPOSE_FILE"

docker network inspect "$NETWORK_NAME" >/dev/null 2>&1 || docker network create "$NETWORK_NAME"
attach_existing_container linguaquest-redis true
attach_existing_container linguaquest-rabbitmq false

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config --quiet
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" pull
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps

echo "LinguaQuest deployment completed from $DEPLOY_DIR."
