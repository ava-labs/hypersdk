#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.unit.sh ]]; then
  echo "must be run from hypersdk root"
  exit 255
fi

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
# shellcheck source=/scripts/constants.sh
source "$HYPERSDK_PATH"/scripts/constants.sh

# Provision of the list of tests requires word splitting, so disable the shellcheck
# shellcheck disable=SC2046
go test -race -timeout="10m" -coverprofile="coverage.out" -covermode="atomic" $(find . -name "*.go" | grep -v "./x/programs/cmd" | grep -v "./examples/morpheusvm" | xargs -n1 dirname | sort -u | xargs)
