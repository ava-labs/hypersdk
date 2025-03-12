#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

git add --all
git update-index --really-refresh >> /dev/null

# Exits if any uncommitted changes are found.
git diff-index --quiet HEAD
