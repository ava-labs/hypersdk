#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.
# Ignore warnings about variables appearing unused since this file is not the consumer of the variables it defines.
# shellcheck disable=SC2034

set -euo pipefail

AVALANCHE_VERSION=${AVALANCHE_VERSION:-'6e7a7d51954c094e4b748748ad1fb1a36b4e5695'}
# Optionally specify a separate version of AvalancheGo for building docker images
# Added to support the case there's no such docker image for the specified commit of AvalancheGo
AVALANCHE_DOCKER_VERSION=${AVALANCHE_DOCKER_VERSION:-'v1.12.2'}

# Shared between ./scripts/build_docker_image.sh and ./scripts/tests.build_docker_image.sh
DEFAULT_VM_NAME="morpheusvm"
DEFAULT_VM_ID="pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O2 -D__BLST_PORTABLE__"
export CGO_ENABLED=1 # Required for cross-compilation
