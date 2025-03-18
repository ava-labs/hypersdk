#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

# This script assumes a linux or nix userland. On macos, invoke `nix develop` before running this script.

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
  pushd "$path" > /dev/null
    go get "github.com/ava-labs/avalanchego@$version"
    go mod tidy
  popd > /dev/null
}

if [[ -n "${VERSION}" ]]; then
  update_avalanchego_mod_version "$PWD" "${VERSION}"
fi

# - Configure constants.sh to determine the full avalanche SHA if the version is not a tag.
# - Determination of the full SHA requires querying the github API and is disabled by default
#   to avoid unnecessary network calls.
# - A full SHA is required for versioning a github custom action e.g. run-monitored-tmpnet-cmd
SET_FULL_AVALANCHE_VERSION=1

# Discover AVALANCHE_VERSION and FULL_AVALANCHE_VERSION
. scripts/constants.sh

# Update AvalancheGo version in examples/morpheusvm
update_avalanchego_mod_version "$PWD/examples/morpheusvm" "$AVALANCHE_VERSION"

# Ensure the custom action version matches the avalanche version
WORKFLOW_PATH=".github/workflows/hypersdk-ci.yml"
for custom_action in "run-monitored-tmpnet-cmd" "install-nix"; do
  echo "Ensuring AvalancheGo version ${FULL_AVALANCHE_VERSION} for ${custom_action} custom action in ${WORKFLOW_PATH} "
  sed -i "s|\(uses: ava-labs/avalanchego/.github/actions/${custom_action}\)@.*|\1@${FULL_AVALANCHE_VERSION}|g" "${WORKFLOW_PATH}"
done

# Ensure the flake version is the same as the avalanche version
FLAKE_DEP="github:ava-labs/avalanchego"
FLAKE_FILE=flake.nix
echo "Ensuring AvalancheGo version ${FULL_AVALANCHE_VERSION} for ${FLAKE_DEP} input of ${FLAKE_FILE}"
sed -i "s|\(${FLAKE_DEP}?ref=\).*|\1${FULL_AVALANCHE_VERSION}\";|g" "${FLAKE_FILE}"
