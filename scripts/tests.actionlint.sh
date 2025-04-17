#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.actionlint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

./bin/actionlint
