#!/bin/bash

set -exu -o pipefail

SCRIPT_DIR=$(dirname "$0")

docker compose -f "$SCRIPT_DIR/compose.yml" up -d
