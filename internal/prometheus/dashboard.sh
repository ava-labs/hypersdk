#!/bin/bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.


set -exu -o pipefail

SCRIPT_DIR=$(dirname "$0")

docker compose -f "$SCRIPT_DIR/compose.yml" up -d
