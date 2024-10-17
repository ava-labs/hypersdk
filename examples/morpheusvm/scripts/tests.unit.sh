#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if [ -n "$1" ]; then
    # Attempt to change to the directory provided as the argument
    if cd "$1"; then
        echo "cd $1"
    else
        echo "failed to cd to directory; $1"
        exit 255
    fi
fi

# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh
# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh

# Provision of the list of tests requires word splitting, so disable the shellcheck
# shellcheck disable=SC2046
go test -race -timeout="10m" -coverprofile="coverage.out" -covermode="atomic" $(find . -name "*.go" | grep -v "./cmd" | grep -v "./tests" | xargs -n1 dirname | sort -u | xargs)
