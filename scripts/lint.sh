#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

if ! [[ "$0" =~ scripts/lint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Default version of golangci-lint
GOLANGCI_LINT_VERSION=${GOLANGCI_LINT_VERSION:-"v1.56.1"}

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/scripts/common/utils.sh
source "$SCRIPT_DIR"/common/utils.sh

if [ "$#" -eq 0 ]; then
  # by default, check all source code
  # to test only "mempool" package
  # ./scripts/lint.sh ./mempool/...
  TARGET="./..."
else
  TARGET="${1}"
fi

add_license_headers -check

# by default, "./scripts/lint.sh" runs all lint tests
TESTS=${TESTS:-"golangci_lint gci"}

# https://github.com/golangci/golangci-lint/releases
function test_golangci_lint {
  go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@"$GOLANGCI_LINT_VERSION"

  golangci-lint run --config .golangci.yml
}

function test_gci {
  install_if_not_exists gci github.com/daixiang0/gci@v0.12.1
  FILES=$(gci list --skip-generated -s standard -s default -s blank -s dot -s "prefix(github.com/ava-labs/hypersdk)" -s alias --custom-order .)
  if [[ "${FILES}" ]]; then
    echo ""
    echo "Some files need to be gci-ed:"
    echo "${FILES}"
    echo ""
    return 1
  fi
}

function run {
  local test="${1}"
  shift 1
  echo "START: '${test}' at $(date)"
  if "test_${test}" "$@" ; then
    echo "SUCCESS: '${test}' completed at $(date)"
  else
    echo "FAIL: '${test}' failed at $(date)"
    exit 255
  fi
}

echo "Running '$TESTS' at: $(date)"
for test in $TESTS; do
  run "${test}" "${TARGET}"
done

echo "ALL SUCCESS!"
