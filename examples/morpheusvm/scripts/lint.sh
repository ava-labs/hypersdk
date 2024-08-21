#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
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

# Specify the version of golangci-lint. Should be upgraded after linting issues are resolved.
export GOLANGCI_LINT_VERSION="v1.51.2"

../../scripts/lint.sh
