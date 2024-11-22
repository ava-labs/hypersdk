#!/bin/bash

set -exu -o pipefail

SCRIPT_DIR=$(dirname "$0")

go run github.com/containerman17/grafana-dashboard-id-remover@4d99b7f "$SCRIPT_DIR/dashboards"
docker compose -f "$SCRIPT_DIR/compose.yml" up -d
