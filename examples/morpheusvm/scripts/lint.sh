#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
set -e

WORKDIR="${GOPATH}"/src/github.com/ava-labs/hypersdk

cd "$WORKDIR"/examples/morpheusvm

# Specify the version of golangci-lint. Should be upgraded after linting issues are resolved.
export GOLANGCI_LINT_VERSION="v1.51.2"

"$WORKDIR"/scripts/lint.sh "$@"
