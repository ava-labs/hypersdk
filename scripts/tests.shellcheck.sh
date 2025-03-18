#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -exuo pipefail

# This script can also be used to correct the problems detected by shellcheck by invoking as follows:
#
# ./scripts/tests.shellcheck.sh -f diff | git apply
#

if ! [[ "$0" =~ scripts/tests.shellcheck.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# `find *` is the simplest way to ensure find does not include a
# leading `.` in filenames it emits. A leading `.` will prevent the
# use of `git apply` to fix reported shellcheck issues. This is
# compatible with both macos and linux (unlike the use of -printf).
#
# shellcheck disable=SC2035
find * -name "*.sh" -type f -print0 | xargs -0 shellcheck "$@"
