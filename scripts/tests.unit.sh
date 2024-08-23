#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.unit.sh ]]; then
  echo "must be run from hypersdk root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}"/constants.sh

file_args=()
while IFS= read -r line; do
    file_args+=("$line")
done < <(find . -type f -name "*.go" | grep -v "./x/programs/cmd" | grep -v "./examples/morpheusvm" | xargs -n1 dirname | sort -u)

go test -race -timeout="10m" -coverprofile="coverage.out" -covermode="atomic" "${file_args[@]}"
