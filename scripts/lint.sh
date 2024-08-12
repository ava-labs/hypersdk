#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

if ! [[ "$0" =~ scripts/lint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Default version of golangci-lint
GOLANGCI_LINT_VERSION=${GOLANGCI_LINT_VERSION:-"v1.56.1"}

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
# shellcheck source=/scripts/common/utils.sh
source "$HYPERSDK_PATH"/scripts/common/utils.sh

if [ "$#" -eq 0 ]; then
  # by default, check all source code
  # to test only "mempool" package
  # ./scripts/lint.sh ./mempool/...
  TARGET="./..."
else
  TARGET="${1}"
fi

# by default, "./scripts/lint.sh" runs all lint tests
# to run only "license_header" test
# TESTS='license_header' ./scripts/lint.sh
TESTS=${TESTS:-"golangci_lint gci license_header"}

# https://github.com/golangci/golangci-lint/releases
function test_golangci_lint {
  go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@"$GOLANGCI_LINT_VERSION"

  # alert the user if they do not have $GOPATH properly configured
  check_command golangci-lint

  golangci-lint run --config .golangci.yml
}

# automatically checks license headers
# to modify the file headers (if missing), remove "--verify" flag
# TESTS='license_header' ADDLICENSE_FLAGS="--verify --debug" ./scripts/lint.sh
_addlicense_flags=${ADDLICENSE_FLAGS:-"--verify --debug"}
function test_license_header {
  go install -v github.com/palantir/go-license@v1.25.0

  # alert the user if they do not have $GOPATH properly configured
  check_command go-license

  local files=()
  while IFS= read -r line; do files+=("$line"); done < <(find . -type f -name '*.go' ! -name '*.pb.go' ! -name 'mock_*.go')

  # shellcheck disable=SC2086
  go-license \
  --config ./license.yml \
  ${_addlicense_flags} \
  "${files[@]}"
}

function test_gci {
  go install -v github.com/daixiang0/gci@v0.12.1
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
