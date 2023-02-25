#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
set -e

if ! [[ "$0" =~ scripts/tests.disk.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

go install -v github.com/cunnie/gobonniego/gobonniego@3ef1d09d1be6d0aa7c760600be80e111393d8ed1
gobonniego -v --runs 3 --size 100 --iops-duration=60
