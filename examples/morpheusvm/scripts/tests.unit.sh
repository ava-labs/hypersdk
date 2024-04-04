#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.unit.sh ]]; then
  echo "must be run from morpheusvm root"
  exit 255
fi

# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh

../../scripts/tests.unit.sh
