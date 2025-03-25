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
  pushd "$path" > /dev/null
    go get "github.com/ava-labs/avalanchego@$version"
    go mod tidy
  popd > /dev/null
}

if [[ -n "${VERSION}" ]]; then
  update_avalanchego_mod_version "$PWD" "${VERSION}"
fi

# Discover AVALANCHE_VERSION
. scripts/constants.sh

# Update AvalancheGo version in examples/morpheusvm
update_avalanchego_mod_version "$PWD/examples/morpheusvm" "$AVALANCHE_VERSION"

# The full SHA is required for versioning custom actions.
CURL_ARGS=(curl -s)
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  # Using an auth token avoids being rate limited when run in CI
  CURL_ARGS+=(-H "Authorization: token ${GITHUB_TOKEN}")
fi
CURL_URL="https://api.github.com/repos/ava-labs/avalanchego/commits/${AVALANCHE_VERSION}"
FULL_AVALANCHE_VERSION="$("${CURL_ARGS[@]}" "${CURL_URL}" | grep '"sha":' | head -n1 | cut -d'"' -f4)"

# Ensure the custom action version matches the avalanche version
WORKFLOW_PATH=".github/workflows/hypersdk-ci.yml"
CUSTOM_ACTION="ava-labs/avalanchego/.github/actions/run-monitored-tmpnet-cmd"
echo "Ensuring AvalancheGo version ${FULL_AVALANCHE_VERSION} for ${CUSTOM_ACTION} custom action in ${WORKFLOW_PATH} "
sed -i.bak "s|\(uses: ${CUSTOM_ACTION}\)@.*|\1@${FULL_AVALANCHE_VERSION}|g" "${WORKFLOW_PATH}" && rm -f "${WORKFLOW_PATH}.bak"
