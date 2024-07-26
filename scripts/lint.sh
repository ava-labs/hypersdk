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

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
# shellcheck source=/scripts/common/utils.sh
source "$HYPERSDK_PATH"/scripts/common/utils.sh

# The -P option is not supported by the grep version installed by
# default on macos. Since `-o errexit` is ignored in an if
# conditional, triggering the problem here ensures script failure when
# using an unsupported version of grep.
grep -P 'lint.sh' scripts/lint.sh &> /dev/null || (\
  >&2 echo "error: This script requires a recent version of gnu grep.";\
  >&2 echo "       On macos, gnu grep can be installed with 'brew install grep'.";\
  >&2 echo "       It will also be necessary to ensure that gnu grep is available in the path.";\
  exit 255 )

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
TESTS=${TESTS:-"golangci_lint license_header require_error_is_no_funcs_as_params single_import interface_compliance_nil require_no_error_inline_func"}

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

function test_single_import {
  if grep -R -zo -P 'import \(\n\t".*"\n\)' .; then
    echo ""
    return 1
  fi
}

function test_require_error_is_no_funcs_as_params {
  if grep -R -zo -P 'require.ErrorIs\(.+?\)[^\n]*\)\n' .; then
    echo ""
    return 1
  fi
}

function test_require_no_error_inline_func {
  if grep -R -zo -P '\t+err :?= ((?!require|if).|\n)*require\.NoError\((t, )?err\)' .; then
    echo ""
    echo "Checking that a function with a single error return doesn't error should be done in-line."
    echo ""
    return 1
  fi
}

# Ref: https://go.dev/doc/effective_go#blank_implements
function test_interface_compliance_nil {
  if grep -R -o -P '_ .+? = &.+?\{\}' .; then
    echo ""
    echo "Interface compliance checks need to be of the form:"
    echo "  var _ json.Marshaler = (*RawMessage)(nil)"
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
RUN=
for test in $TESTS; do
  RUN="${RUN} run ${test} &"
done

RUN=${RUN%&}

(trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT; eval ${RUN} wait)
