#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

CFMMVM_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

ginkgo -v "$CFMMVM_PATH"/tests/e2e/e2e.test -- --stop-network
