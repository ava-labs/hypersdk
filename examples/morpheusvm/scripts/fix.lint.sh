#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
set -e

WORKDIR="${GOPATH}"/src/github.com/ava-labs/hypersdk

"${WORKDIR}"/scripts/fix.lint.sh
