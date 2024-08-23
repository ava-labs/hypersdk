#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.actionlint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/scripts/common/utils.sh
source "$SCRIPT_DIR"/common/utils.sh

install_if_not_exists actionlint github.com/rhysd/actionlint/cmd/actionlint@v1.7.1

actionlint
