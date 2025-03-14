#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

git add --all
git update-index --really-refresh >> /dev/null

# Check if there are uncommitted changes
if ! git diff-index --quiet HEAD; then
    echo "ERROR: Uncommitted changes found:"
    echo "----------------------------------------"
    git diff --name-status HEAD | cat
    echo "----------------------------------------"
    echo "Run 'git diff' to see detailed changes"
    exit 1
fi
