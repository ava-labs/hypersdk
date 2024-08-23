#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/tests.actionlint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR"/common/utils.sh

go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.1

check_command actionlint

actionlint
