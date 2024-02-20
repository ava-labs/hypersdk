#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit

if ! [[ "$0" =~ scripts/fix.lint.sh ]]; then
  echo "must be run from tokenvm root"
  exit 255
fi

../../scripts/fix.lint.sh
