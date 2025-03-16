#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

if ! [[ "$0" =~ scripts/update_avalanchego_version.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# If version is not provided, the existing version in go.mod is assumed
VERSION="${1:-}"

# Function to update version in go.mod and run go get
update_avalanchego_mod_version() {
    local path=$1
    local version=$2

    echo "Ensuring AvalancheGo version $version in $path/go.mod"
    (cd "$path" && go get "github.com/ava-labs/avalanchego@$version" && go mod tidy)
}

if [[ -n "${VERSION}" ]]; then
  echo "Ensuring AvalancheGo version to $VERSION in go.mod"
  update_avalanchego_mod_version "$PWD" "${VERSION}"
fi

# - Configure constants.sh to determine the full avalanche SHA if the version is not a tag.
# - Determination of the full SHA requires querying the github API and is disabled by default
#   to avoid unnecessary network calls.
# - A full SHA is required for versioning a github custom action like the run-monitored-tmpnet-cmd
SET_FULL_AVALANCHE_VERSION=1

# Discover AVALANCHE_VERSION and FULL_AVALANCHE_VERSION
. scripts/constants.sh

# Update AvalancheGo version in examples/morpheusvm
update_avalanchego_mod_version "$PWD/examples/morpheusvm" "$AVALANCHE_VERSION"

# Ensure the custom action version matches the avalanche version
WORKFLOW_PATH=".github/workflows/hypersdk-ci.yml"
CUSTOM_ACTION="ava-labs/avalanchego/.github/actions/run-monitored-tmpnet-cmd"
echo "Ensuring AvalancheGo version ${FULL_AVALANCHE_VERSION} in ${WORKFLOW_PATH} for ${CUSTOM_ACTION}"
sed -i "s|\(uses: ${CUSTOM_ACTION}\)@.*|\1@${FULL_AVALANCHE_VERSION}|g" "${WORKFLOW_PATH}"

# Ensure the flake version is the same as the avalanche version
FLAKE_DEP="github:ava-labs/avalanchego"
FLAKE_FILE=flake.nix
echo "Ensuring AvalancheGo version ${FULL_AVALANCHE_VERSION} for ${FLAKE_DEP} in ${FLAKE_FILE}"
sed -i "s|\(${FLAKE_DEP}?ref=\).*|\1${FULL_AVALANCHE_VERSION}\";|g" "${FLAKE_FILE}"
