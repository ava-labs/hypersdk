#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.benchmark.sh ]]; then
  echo "must be run from hypersdk root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/scripts/constants.sh
source "${SCRIPT_DIR}"/constants.sh

subdir=${1:-.}

file_args=()
while IFS= read -r line; do
    file_args+=("$line")
done < <(find "$subdir" -type f -name "*.go" | grep -v "./examples/" | xargs -n1 dirname | sort -u)

go test -timeout="10m" -bench=. -benchtime=1x "${file_args[@]}"
