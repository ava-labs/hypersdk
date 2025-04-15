#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )
"${REPO_ROOT}"/scripts/run_versioned_binary.sh google.golang.org/protobuf/cmd/protoc-gen-go v1.33.0 "${@}"
