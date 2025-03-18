#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.
# Ignore warnings about variables appearing unused since this file is not the consumer of the variables it defines.
# shellcheck disable=SC2034

if [[ -z ${AVALANCHE_VERSION:-} ]]; then
  # Get module details from go.mod
  MODULE_DETAILS="$(go list -m "github.com/ava-labs/avalanchego" 2>/dev/null)"

  # Extract the version part
  AVALANCHE_VERSION="$(echo "${MODULE_DETAILS}" | awk '{print $2}')"

  # Check if the version matches the pattern where the last part is the module hash
  # v*YYYYMMDDHHMMSS-abcdef123456
  #
  # If not, the value is assumed to represent a tag
  if [[ "${AVALANCHE_VERSION}" =~ ^v.*[0-9]{14}-[0-9a-f]{12}$ ]]; then
    # Extract module hash from version
    MODULE_HASH="$(echo "${AVALANCHE_VERSION}" | cut -d'-' -f3)"

    # The first 8 chars of the hash is used as the tag of avalanchego images
    AVALANCHE_VERSION="${MODULE_HASH::8}"
  fi
fi

# Optionally specify a separate version of AvalancheGo for building docker images
# Added to support the case there's no such docker image for the specified commit of AvalancheGo
AVALANCHE_DOCKER_VERSION=${AVALANCHE_DOCKER_VERSION:-'v1.12.2'}

# Shared between ./scripts/build_docker_image.sh and ./scripts/tests.build_docker_image.sh
DEFAULT_VM_NAME="morpheusvm"
# This needs to match the ID defined in examples/morpheusvm/consts/consts.go
DEFAULT_VM_ID="qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u"

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O2 -D__BLST_PORTABLE__"
export CGO_ENABLED=1 # Required for cross-compilation
