#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

# Check if a version argument is provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

if ! [[ "$0" =~ scripts/update_avalanchego_version.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

VERSION=$1

# Function to update version in go.mod and run go get
update_avalanchego_mod_version() {
    local path=$1
    local version=$2

    # Set the working directory to the provided path and update the AvalancheGo dependency
    (cd "$path" && go get "github.com/ava-labs/avalanchego@v$version")
}

# Funciont to update the version in the format "VERSION=vXX.XX.XX" in the provided file
# Intended to run on the given run.sh files for each example VM.
update_avalanchego_run_version() {
    local file_path=$1
    local version=$2

    # Use sed to find and replace the version in the file
    # macOS requires an empty string after -i, but this may not work on Linux
    sed -i '' "s/^VERSION=v[0-9]*\.[0-9]*\.[0-9]*/VERSION=v$version/" "$file_path"
}

PWD=$(PWD)
# Update version in the root directory
update_avalanchego_mod_version "$PWD" "$VERSION"

# Update AvalancheGo version in examples/morpheusvm
update_avalanchego_mod_version "$PWD/examples/morpheusvm" "$VERSION"
update_avalanchego_run_version "$PWD/examples/morpheusvm/scripts/run.sh" "$VERSION"

# Update AvalancheGo version in examples/tokenvm
update_avalanchego_mod_version "$PWD/examples/tokenvm" "$VERSION"
update_avalanchego_run_version "$PWD/examples/tokenvm/scripts/run.sh" "$VERSION"  
