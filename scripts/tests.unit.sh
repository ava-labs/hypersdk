#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.unit.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
source "$HYPERSDK_PATH"/scripts/constants.sh

# Provision of the list of tests requires word splitting, so disable the shellcheck
# shellcheck disable=SC2046
go test -race -timeout="3m" -coverprofile="coverage.out" -covermode="atomic" $(find . -name "*.go" | grep -v "./x/programs/cmd" | grep -v "./examples/morpheusvm" | grep -v "./examples/tokenvm" | xargs -n1 dirname | sort -u | xargs)
